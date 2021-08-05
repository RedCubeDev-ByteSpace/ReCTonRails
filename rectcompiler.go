package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"
)

func EvaluateRoR(code string, file string, w http.ResponseWriter, r *http.Request) string {
	if !shut {
		fmt.Println("[RoR] Serving RoRHTML page...  [ " + file + " ]")
	}

	uuid := uuid.NewV4().String()
	h := sha1.New()
	h.Write([]byte(code))
	nHash := h.Sum(nil)

	dir := "./cache/" + strings.ReplaceAll(file, "/", "#")

	//create chache dir if non-existant
	if _, err := os.Stat("./cache"); os.IsNotExist(err) {
		os.Mkdir("./cache", os.ModePerm)
		os.Mkdir("./cache/upload", os.ModePerm)
	}

	//if page cached, check hash if file didnt change return if it did recompile
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		hash, _ := os.ReadFile(dir + "/hash")

		if string(hash) == string(nHash) {
			if !shut {
				fmt.Println("[RoR] Cache is still up to date. No Compiling required, serving page...  [ " + dir + "/binary.dll" + " ]")
			}
			return SlotInResults(code, ReplaceDataAndRunBinary(uuid, r, dir, string(nHash)), uuid, w, dir)
		}
		if !shut {
			fmt.Println("[RoR] Cache not up to date. Recompiling required.")
		}
	}

	var snippets []string

	snippetRegex := regexp.MustCompile(`#{[\s\S]+?}#`)
	snippetMatches := snippetRegex.FindAllString(code, -1)

	if snippetMatches != nil {
		for i := 0; i < len(snippetMatches); i++ {
			snippets = append(snippets, strings.TrimSuffix(strings.TrimPrefix(snippetMatches[i], "#{"), "}#"))
		}
	}

	//clear directory
	os.RemoveAll(dir)
	os.Mkdir(dir, os.ModePerm)

	hasError, output := CompileReCTCode(AssembleReCTCode(snippets, uuid, r, dir), file, dir)

	if hasError {
		return output
	}

	if !shut {
		fmt.Println("[RoR] Creating Cache and serving Page...  [ " + dir + " ]")
	}

	return SlotInResults(code, ReplaceDataAndRunBinary(uuid, r, dir, string(nHash)), uuid, w, dir)
}

func AssembleReCTCode(snippets []string, uuid string, r *http.Request, dir string) string {
	//watch := stopwatch.Start()
	code := "package io; var cwd <- io::GetCurrentDirectory(); io::ChangeDirectory(cwd+\"/" + dir + "\");\n #attach(\"./../../boilerplate.rct\"); \n io::ChangeDirectory(cwd); \n"

	for i := 0; i < len(snippets); i++ {
		code += "\n /*INTERNAL*/ Write(private_uuid + \"+ \"); \n"
		code += snippets[i]
		code += "\n /*INTERNAL*/ Write(private_uuid + \".\"); \n"
	}

	//watch.Stop()
	//color.Cyan("[Debug] Function \"AssembleReCTCode\" took: " + fmt.Sprint(watch.Seconds().Nanoseconds()) + " Seconds\n")

	return code
}

func CompileReCTCode(code string, url string, dir string) (bool, string) {
	//watch := stopwatch.Start()
	if !shut {
		fmt.Println("[RoR] Compiling...  [ " + dir + "/code.rct" + " ]")
	}

	os.WriteFile(dir+"/code.rct", []byte(code), 0644)

	rectPath := "rctc"

	if os.Getenv("RCTC_PATH") != "" {
		rectPath = os.Getenv("RCTC_PATH")
	}

	cmd := exec.Command(rectPath, dir+"/code.rct", "-s", "-f", "-o", dir+"/binary.dll")
	out, _ := cmd.CombinedOutput()

	if _, err := os.Stat(dir + "/binary.dll"); err != nil {
		//watch.Stop()
		//color.Cyan("[Debug] Function \"CompileReCTCode\" took: " + fmt.Sprint(watch.Seconds().Nanoseconds()) + " Seconds\n")

		return true, "RoR Log: \n\n" + string(out)
	}

	//watch.Stop()
	//color.Cyan("[Debug] Function \"CompileReCTCode\" took: " + fmt.Sprint(watch.Seconds().Nanoseconds()) + " Seconds\n")

	return false, string(out)
}

func ReplaceDataAndRunBinary(uuid string, r *http.Request, dir string, hash string) string {
	//watch := stopwatch.Start()
	os.WriteFile(dir+"/essentials", []byte(r.Method+"\n"+r.URL.Path+"\n"+uuid), 0644)
	os.Mkdir("./cache/upload/"+strings.ReplaceAll(uuid, "-", ""), os.ModePerm)

	FormFile := ""
	FileFile := ""
	CookieFile := ""

	//normal HTML Form
	r.ParseForm()
	//mutlipart Form
	r.ParseMultipartForm(32 << 20) //32MB of memory
	for k := range r.Form {
		FormFile += k + uuid + r.FormValue(k) + uuid
	}

	//mutlipart Form
	if r.MultipartForm != nil {
		for k := range r.MultipartForm.File {
			file, header, err := r.FormFile(k)

			if err != nil {
				continue
			}

			var buf bytes.Buffer
			io.Copy(&buf, file)

			fnparts := strings.Split(header.Filename, ".")
			os.WriteFile("./cache/upload/"+strings.ReplaceAll(uuid, "-", "")+"/"+k+"."+fnparts[len(fnparts)-1], buf.Bytes(), 0644)
			FileFile += k + uuid + "./cache/upload/" + strings.ReplaceAll(uuid, "-", "") + "/" + k + "." + fnparts[len(fnparts)-1] + uuid
		}
	}

	//cookies
	cookies := r.Cookies()
	for i := 0; i < len(cookies); i++ {
		CookieFile += cookies[i].Name + uuid + cookies[i].Value + uuid
	}

	os.WriteFile(dir+"/Form", []byte(FormFile), 0644)
	os.WriteFile(dir+"/Files", []byte(FileFile), 0644)
	os.WriteFile(dir+"/Cookies", []byte(CookieFile), 0644)
	os.WriteFile(dir+"/hash", []byte(hash), 0644)

	app := exec.Command("dotnet", dir+"/binary.dll")
	appout, err := app.CombinedOutput()

	if err != nil {
		os.RemoveAll("./cache/upload/" + strings.ReplaceAll(uuid, "-", "") + "/")

		//watch.Stop()
		//color.Cyan("[Debug] Function \"ReplaceDataAndRunBinary\" took: " + fmt.Sprint(watch.Seconds().Nanoseconds()) + " Seconds\n")

		return "RoR Log: \n\n" + string(appout) + "\n\n" + err.Error()
	}

	os.RemoveAll("./cache/upload/" + strings.ReplaceAll(uuid, "-", "") + "/")

	//watch.Stop()
	//color.Cyan("[Debug] Function \"ReplaceDataAndRunBinary\" took: " + fmt.Sprint(watch.Seconds().Nanoseconds()) + " Seconds\n")

	return string(appout)
}

func RunReCTCode(code string, uuid string) string {
	//watch := stopwatch.Start()
	os.WriteFile("./cache/rorcode.rct", []byte(code), 0644)
	cmd := exec.Command("rctc", "./cache/rorcode.rct", "-s", "-f", "-o", "./cache/rorcom.dll")
	out, _ := cmd.CombinedOutput()

	if _, err := os.Stat("./cache/rorcom.dll"); err != nil {
		return "RoR Log: \n\n" + string(out)
	}

	app := exec.Command("dotnet", "./cache/rorcom.dll")
	appout, _ := app.Output()

	//watch.Stop()
	//color.Cyan("[Debug] Function \"RunReCTCode\" took: " + fmt.Sprint(watch.Seconds().Nanoseconds()) + " Seconds\n")

	return string(appout)
}

func SlotInResults(source string, result string, uuid string, w http.ResponseWriter, dir string) string {
	os.WriteFile(dir+"/rawout", []byte(result), 0644)

	if strings.HasPrefix(result, "RoR Log: \n\n") {
		return result
	}

	//watch := stopwatch.Start()

	var snippets []string

	snippetRegex := regexp.MustCompile(`#{[\s\S]+?}#`)
	snippetMatches := snippetRegex.FindAllString(source, -1)

	if snippetMatches != nil {
		for i := 0; i < len(snippetMatches); i++ {
			snippets = append(snippets, snippetMatches[i])
		}
	}

	//watch.Stop()
	//color.Green("[Debug] Part \"GetSnippets\" took: " + fmt.Sprint(watch.Seconds().Nanoseconds()) + " Seconds\n")
	//watch.Start()

	var slotins []string

	slotinRegex := regexp.MustCompile(uuid + `\+([\s\S]+?)?` + uuid + `\.`)
	slotinMatches := slotinRegex.FindAllString(result, -1)

	if slotinMatches != nil {
		for i := 0; i < len(slotinMatches); i++ {
			slotins = append(slotins, strings.TrimSuffix(strings.TrimPrefix(slotinMatches[i], uuid+"+"), uuid+"."))
		}
	}

	//watch.Stop()
	//color.Green("[Debug] Part \"GetSlotins\" took: " + fmt.Sprint(watch.Seconds().Nanoseconds()) + " Seconds\n")
	//watch.Start()

	inSnippit := false
	lastSlotin := ""

	if strings.Contains(result, uuid+";") {
		cookiename := ""
		cookievalue := ""
		cookiedeath := 0

		for i := 36; i < len(result); i++ {
			if !inSnippit {
				if string([]rune(result)[i]) == ";" {
					if result[i-36:i] == uuid {
						inSnippit = true
						continue
					}
				}
			} else {
				if string([]rune(result)[i]) == "," {
					if result[i-36:i] == uuid {
						cookiename = lastSlotin[0 : len(lastSlotin)-36]
						lastSlotin = ""
						continue
					}
				}
				if string([]rune(result)[i]) == ":" {
					if result[i-36:i] == uuid {
						cookievalue = lastSlotin[0 : len(lastSlotin)-36]
						lastSlotin = ""
						continue
					}
				}
				if string([]rune(result)[i]) == ";" {
					if result[i-36:i] == uuid {
						cookiedeath, _ = strconv.Atoi(lastSlotin[0 : len(lastSlotin)-36])
						snippets = append(snippets, uuid+";"+cookiename+uuid+","+cookievalue+uuid+":"+lastSlotin[0:len(lastSlotin)-36]+uuid+";")
						slotins = append(slotins, "")
						inSnippit = false
						lastSlotin = ""
						http.SetCookie(w, &http.Cookie{Name: cookiename, Value: cookievalue, Expires: time.Now().Add(time.Second * time.Duration(cookiedeath))})
						continue
					}
				}
				lastSlotin += string([]rune(result)[i])
			}
		}
	}

	//watch.Stop()
	//color.Green("[Debug] Part \"PlonkInSlotins\" took: " + fmt.Sprint(watch.Seconds().Nanoseconds()) + " Seconds\n")
	//watch.Start()

	for i := 0; i < len(slotins); i++ {
		source = strings.Replace(source, snippets[i], slotins[i], 1)
	}

	if strings.Contains(source, uuid+"!") {
		parts := strings.Split(source, uuid+"!")
		return parts[0]
	}

	//watch.Stop()
	//color.Cyan("[Debug] Function \"SlotInResults\" took: " + fmt.Sprint(watch.Seconds().Nanoseconds()) + " Seconds\n")

	return source
}
