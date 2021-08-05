package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

var wasPiped bool = false
var starPrefixes []string
var shut bool = false

func main() {
	fmt.Println("ReCT On Rails! v1.2  --  A Web-Framework for ReCT")

	port := "8080"

	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}

	fmt.Println("Listening on Port " + port)

	fs := http.FileServer(http.Dir("www/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	var fss []http.Handler

	if os.Getenv("PLZ_HOST") != "" {
		hosts := strings.Split(os.Getenv("PLZ_HOST"), ";")

		for i := 0; i < len(hosts); i++ {
			parts := strings.Split(hosts[i], ":")
			fmt.Println("Hosting '" + parts[0] + "' at'" + parts[1] + "'...")
			sfs := http.FileServer(http.Dir(parts[0]))
			http.Handle(parts[1], http.StripPrefix(parts[1], sfs))
			fss = append(fss, sfs)
		}
	}

	//pump all requests over here
	http.HandleFunc("/", requestHandler)

	//parse pipes to find "any" selectors
	fmt.Println("Parsing connections.pipes and caching * selectors...")
	pipeBuf, _ := os.ReadFile("connections.pipes")
	pipes := strings.Split(string(pipeBuf), "\n")

	for i := 0; i < len(pipes); i++ {
		if pipes[i] == "" || strings.HasPrefix(pipes[i], "#") {
			continue
		}
		pipe := strings.Split(pipes[i], " ")
		if strings.HasSuffix(pipe[1], "*") && len(pipe[1]) > 1 {
			starPrefixes = append(starPrefixes, strings.TrimSuffix(pipe[1], "*"))
		}
	}

	if len(os.Args) > 1 {
		if os.Args[1] == "--shut" {
			shut = true
			fmt.Println("Will not print debug messages.")
		}
	}

	//start http server
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func requestHandler(w http.ResponseWriter, r *http.Request) {
	wasPiped = false
	fmt.Fprint(w, string(resolveRequest(r.URL.Path, w, r)))
}

func resolveRequest(url string, w http.ResponseWriter, r *http.Request) []byte {
	pipeBuf, err := os.ReadFile("connections.pipes")

	if err != nil {
		fmt.Println("[Router][Error] connections.pipes file not found!")
		return []byte("500 - Internal Server Error")
	}

	if _, err := os.Stat("www" + url); err == nil {
		buf, err := os.ReadFile("www" + url)

		//if piping for files is requested, pipe
		if strings.HasPrefix(string(pipeBuf), "#pipe_existing_files=true") && !wasPiped {
			goto pipeMe
		}

		//if file was found -> return
		if err == nil {
			if strings.HasSuffix(url, ".rorhtml") {
				//watch := stopwatch.Start()
				rorres := EvaluateRoR(string(buf), url, w, r)
				//watch.Stop()
				//color.Red("[Debug] Function \"EvaluateRoR\" took: " + fmt.Sprint(watch.Seconds().Nanoseconds()) + " Seconds\n")
				return []byte(rorres)
			}
			return buf
		}

	}

	//The print explains it...
	if !shut {
		fmt.Println("[Router] File does not exist [" + url + "], checking pipes...")
	}

pipeMe:

	//Read the connections.pipes file
	pipeFile := string(pipeBuf)
	pipes := strings.Split(pipeFile, "\n")
	pipeMap := make(map[string]string)

	//parse the file
	for i := 0; i < len(pipes); i++ {
		if pipes[i] == "" || strings.HasPrefix(pipes[i], "#") {
			continue
		}
		pipe := strings.Split(pipes[i], " ")
		pipeMap[pipe[1]] = pipe[3]
	}

	//check for star pipe
	starPipe, ok := pipeMap["*"]
	if ok {
		wasPiped = true
		return resolveRequest(starPipe, w, r)
	}

	//check for prefixed star pipe
	for i := 0; i < len(starPrefixes); i++ {
		if strings.HasPrefix(url, starPrefixes[i]) {
			preStarPipe, ok := pipeMap[starPrefixes[i]+"*"]

			if ok {
				wasPiped = true
				return resolveRequest(preStarPipe, w, r)
			}
		}
	}

	usePipe, ok := pipeMap[url]

	//If pipe not found move to error pipe, if it was looking for the error pipe already then return error
	if !ok {
		if url == "?" {
			return []byte("404 - File not found")
		}
		if !shut {
			fmt.Println("[Router] Couldnt find Pipe! Redirecting to Error Pipe...")
		}
		return resolveRequest("?", w, r)
	}
	if !shut {
		fmt.Println("[Router] Found Pipe! piping... [" + url + " --> " + usePipe + "]")
	}
	wasPiped = true
	return resolveRequest(usePipe, w, r)
}
