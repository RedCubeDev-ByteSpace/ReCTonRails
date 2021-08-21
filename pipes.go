package main

import (
	"fmt"
	"os"
	"strings"
)

func ParsePipes() map[string]*Domain {
	var domains = make(map[string]*Domain)
	buffer, err := os.ReadFile("connections.pipes")

	if err != nil {
		fmt.Println("[Router][Error] connections.pipes file not found!")
		return nil
	}

	fileString := string(buffer)
	lines := strings.Split(fileString, "\n")

	inDomain := "default"
	domains["default"] = &Domain{make(map[string]string), make([][]string, 0), "", ""}

	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "#") {
			continue
		}

		lines[i] = strings.TrimLeft(lines[i], " \t")
		if lines[i] == "" {
			continue
		}

		parts := strings.Split(lines[i], " ")
		if parts[0] == "pipe" {
			if parts[1] == "*" {
				domains[inDomain].AllPipe = parts[3]
				continue
			}

			if parts[1] == "?" {
				domains[inDomain].ErrorPipe = parts[3]
				continue
			}

			if strings.HasSuffix(parts[1], "*") {
				domains[inDomain].Wildcards = append(domains[inDomain].Wildcards, []string{strings.TrimSuffix(parts[1], "*"), parts[3]})
			} else {
				domains[inDomain].Pipes[parts[1]] = parts[3]
			}
		}

		if parts[0] == "domain" {
			inDomain = parts[1]
			domains[inDomain] = &Domain{make(map[string]string), make([][]string, 0), "", ""}
		}

		if parts[0] == "end" {
			inDomain = "default"
		}
	}

	return domains
}

type Domain struct {
	Pipes     map[string]string
	Wildcards [][]string
	AllPipe   string
	ErrorPipe string
}
