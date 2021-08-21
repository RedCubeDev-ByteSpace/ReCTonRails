package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/fatih/color"
)

var wasPiped bool = false
var starPrefixes []string
var shut bool = false
var ssl bool = false
var nocache bool = false
var pipeCounter int = 0

func main() {
	fmt.Println("ReCT On Rails! v1.3  --  A Web-Framework for ReCT")

	for i := 0; i < len(os.Args); i++ {
		if os.Args[i] == "--shut" {
			shut = true
		} else if os.Args[i] == "--useSSL" {
			ssl = true
		} else if os.Args[i] == "--noCache" {
			nocache = true
			color.Red("Caching is DISBALED! (this is a debug feature, do NOT use this on a production site! It will cause massive slowdowns.)")
		}
	}

	port := "8080"
	sslport := "443"

	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}

	if os.Getenv("SSLPORT") != "" {
		sslport = os.Getenv("SSLPORT")
	}

	fmt.Println("Listening on Port " + port)

	if ssl {
		fmt.Println("Listening on Port " + sslport + " (HTTPS)...")

		if _, err := os.Stat("./ssl/server.key"); err != nil {
			color.Red("Couldnt find file './ssl/server.key' (and started in SSL Mode)! Aborting...")
			os.Exit(-1)
		}

		if _, err := os.Stat("./ssl/server.key"); err != nil {
			color.Red("Couldnt find file './ssl/server.crt' (and started in SSL Mode)! Aborting...")
			os.Exit(-1)
		}
	}

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

	//start http server
	if !ssl {
		color.Yellow("Your server does not have SSL / HTTPS! You can find more info on how to use SSL on the RoR Docs.")
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatal(err)
		}
	} else {
		go redirectThread(port)
		if err := http.ListenAndServeTLS(":"+sslport, "./ssl/server.crt", "./ssl/server.key", nil); err != nil {
			log.Fatal(err)
		}
	}

}

func redirectThread(port string) {
	fmt.Println("Starting HTTP redirect server... (forcing HTTPS)")
	if err := http.ListenAndServe(":"+port, http.HandlerFunc(redirectSSL)); err != nil {
		log.Fatal(err)
	}
}

func redirectSSL(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusMovedPermanently)
}

func requestHandler(w http.ResponseWriter, r *http.Request) {
	wasPiped = false
	pipeCounter = 0
	fmt.Fprint(w, string(resolveRequest(r.URL.Path, w, r)))
}

func resolveRequest(url string, w http.ResponseWriter, r *http.Request) []byte {
	pipeCounter++

	if pipeCounter > 10 {
		return []byte("500 - Internal Server Error! Found endless Loop in Pipes.")
	}

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

	domains := ParsePipes()
	domain, ok := domains[r.Host]

	// choose default domain if there isnt a domain entry
	if !ok {
		domain = domains["default"]
	}

	// check for "star pipe" (*) for redirect of everything
	if domain.AllPipe != "" {
		wasPiped = true
		return resolveRequest(domain.AllPipe, w, r)
	}

	// check all wildcards
	for i := 0; i < len(domain.Wildcards); i++ {
		if strings.HasPrefix(url, domain.Wildcards[i][0]) {
			wasPiped = true
			return resolveRequest(domain.Wildcards[i][1], w, r)
		}
	}

	// check if request is coming from error pipe and search for it
	if url == "?" {
		if domain.ErrorPipe != "" {
			wasPiped = true
			return resolveRequest(domain.ErrorPipe, w, r)
		} else if domains["default"].ErrorPipe != "" {
			wasPiped = true
			return resolveRequest(domains["default"].ErrorPipe, w, r)
		} else {
			return []byte("404 - File not found")
		}
	}

	usePipe, ok := domain.Pipes[url]

	//If pipe not found move to error pipe, if it was looking for the error pipe already then return error
	if !ok {
		if !shut {
			fmt.Println("[Router] Couldnt find Pipe! Redirecting to Error Pipe...")
		}
		return resolveRequest("?", w, r)
	}

	// if pipe exist, pipe it!
	if !shut {
		fmt.Println("[Router] Found Pipe! piping... [" + url + " --> " + usePipe + "]")
	}
	wasPiped = true
	return resolveRequest(usePipe, w, r)
}
