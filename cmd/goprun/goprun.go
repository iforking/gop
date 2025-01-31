package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/goplus/gop/x/gopmod"
	"github.com/goplus/gop/x/gopproj"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, "Usage: goprun package [arguments ...]\n\n")
		return
	}
	proj, args, err := gopproj.ParseOne(os.Args[1:]...)
	if err != nil {
		log.Fatalln(err)
	}
	var ctx = gopmod.New("")
	goProj, err := ctx.OpenProject(0, proj)
	if err != nil {
		fmt.Fprint(os.Stderr, "OpenProject failed:", err)
		return
	}
	goProj.ExecArgs = args
	cmd := ctx.GoCommand("run", goProj)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	err = cmd.Run()
	if err != nil {
		switch e := err.(type) {
		case *exec.ExitError:
			os.Exit(e.ExitCode())
		default:
			log.Fatalln(err)
		}
	}
}
