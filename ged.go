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
)

// current FileBuffer
var buffer *FileBuffer

// current ed state
var state struct {
	fileName string // current filename
	lastErr  error
	printErr bool
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
		if e = exe(ctx); e != nil {
			return
		}
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
		}
	}
	inScan := bufio.NewScanner(os.Stdin)
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
	}
	if inScan.Err() != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v", inScan.Err())
		os.Exit(1)
	}
}
