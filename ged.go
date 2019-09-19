// ged.go - main entry point for ged
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
)

// flags
var (
	fSuppress = flag.Bool("s", false, "suppress counts")
	fPrompt   = flag.String("p", "*", "specify a command prompt")
	fLoose    = flag.Bool("l", false, "loose exit mode, don't return errors for command failure (not implemented)")
	fRestrict = flag.Bool("r", false, "no editing outside directory, no command exec (not implemented)")
)

// current FileBuffer
var buffer *FileBuffer

// current ed state
var state struct {
	fileName string // current filename
	lastErr  error
	printErr bool
	prompt   bool
	winSize  int
	lastRep  string
	lastSub  string
}

// Parse input and run command
func run(cmd string) (e error) {
	ctx := &Context{
		cmd: cmd,
	}
	if ctx.addrs, ctx.cmdOffset, e = buffer.ResolveAddrs(cmd); e != nil {
		return
	}
	if len(cmd) <= ctx.cmdOffset {
		// no command, default to print
		ctx.cmd += "p"
	}
	if exe, ok := cmds[ctx.cmd[ctx.cmdOffset]]; ok {
		buffer.Start()
		if e = exe(ctx); e != nil {
			return
		}
		buffer.End()
	} else {
		return fmt.Errorf("invalid command: %v", cmd[ctx.cmdOffset])
	}
	return
}

// Entry point
func main() {
	var e error
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [-s] [-p <prompt>] [file]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "p" {
			state.prompt = true
		}
	})
	args := flag.Args()
	if len(args) > 1 { // we only accept one additional argument
		flag.Usage()
		os.Exit(1)
	}
	buffer = NewFileBuffer(nil)
	if len(args) == 1 { // we were given a file name
		state.fileName = args[0]
		// try to read in the file
		if _, e = os.Stat(state.fileName); os.IsNotExist(e) && !*fSuppress {
			fmt.Fprintf(os.Stderr, "%s: No such file or directory", state.fileName)
			// this is not fatal, we just start with an empty buffer
		} else {
			if buffer, e = FileToBuffer(state.fileName); e != nil {
				fmt.Fprintln(os.Stderr, e)
				os.Exit(1)
			}
			if !*fSuppress {
				fmt.Println(buffer.Size())
			}
		}
	}
	state.winSize = 22 // we don't actually support getting the real window size
	inScan := bufio.NewScanner(os.Stdin)
	if state.prompt {
		fmt.Printf("%s", *fPrompt)
	}
	for inScan.Scan() {
		cmd := inScan.Text()
		e = run(cmd)
		if e != nil {
			state.lastErr = e
			if !*fSuppress && state.printErr {
				fmt.Println(e)
			} else {
				fmt.Println("?")
			}
		}
		if state.prompt {
			fmt.Printf("%s", *fPrompt)
		}
	}
	if inScan.Err() != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v", inScan.Err())
		os.Exit(1)
	}
}
