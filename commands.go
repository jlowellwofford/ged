// commands.go - defines editor commands
package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
)

// A Context is passed to an invoked command
type Context struct {
	cmd       string // full command string
	cmdOffset int    // start of the command after address resolution
	addrs     []int  // resolved addresses
}

// A Command can be run with a Context and returns an error
type Command func(*Context) error

// The cmds map maps single byte commands to their handler functions.
// This is also a good way to check what commands are implemented.
var cmds = map[byte]Command{
	'q': cmdQuit,
	'Q': cmdQuit,
	'd': cmdDelete,
	'l': cmdPrint,
	'p': cmdPrint,
	'n': cmdPrint,
	'h': cmdErr,
	'H': cmdErr,
	'a': cmdInput,
	'i': cmdInput,
	'c': cmdInput,
	'w': cmdWrite,
	'W': cmdWrite,
	'k': cmdMark,
	'e': cmdEdit,
	'E': cmdEdit,
	'r': cmdEdit,
	'f': cmdFile,
	'=': cmdLine,
	'j': cmdJoin,
	'm': cmdMove,
	'#': func(*Context) (e error) { return },
}

//////////////////////
// Command handlers /
////////////////////

func cmdDelete(ctx *Context) (e error) {
	var r [2]int
	if r, e = buffer.AddrRangeOrLine(ctx.addrs); e != nil {
		return
	}
	e = buffer.Delete(r)
	return
}

func cmdQuit(ctx *Context) (e error) {
	if ctx.cmd[ctx.cmdOffset] == 'q' && buffer.Dirty() {
		return fmt.Errorf("warning: file modified")
	}
	os.Exit(0)
	return
}

func cmdPrint(ctx *Context) (e error) {
	var r [2]int
	if r, e = buffer.AddrRangeOrLine(ctx.addrs); e != nil {
		return
	}
	for l := r[0]; l <= r[1]; l++ {
		if ctx.cmd[ctx.cmdOffset] == 'n' {
			fmt.Printf("%d\t", l+1)
		}
		line := buffer.GetMust(l, true)
		if ctx.cmd[ctx.cmdOffset] == 'l' {
			line += "$" // TODO: the man pages describes more escaping, but it's not clear what GNU ed actually does.
		}
		fmt.Printf("%s\n", line)
	}
	return
}

func cmdErr(ctx *Context) (e error) {
	if ctx.cmd[ctx.cmdOffset] == 'h' {
		if state.lastErr != nil {
			fmt.Println(state.lastErr)
			return
		}
	}
	if ctx.cmd[ctx.cmdOffset] == 'H' {
		if state.printErr {
			state.printErr = false
			return
		}
		state.printErr = true
	}
	return
}

func cmdInput(ctx *Context) (e error) {
	scan := bufio.NewScanner(os.Stdin)
	nbuf := []string{}
	if len(ctx.cmd[ctx.cmdOffset+1:]) != 0 && ctx.cmd[ctx.cmdOffset] != 'c' {
		return fmt.Errorf("%c only takes a single line addres", ctx.cmd[ctx.cmdOffset])
	}
	for scan.Scan() {
		line := scan.Text()
		if line == "." {
			break
		}
		nbuf = append(nbuf, line)
	}
	if len(nbuf) == 0 {
		return
	}
	switch ctx.cmd[ctx.cmdOffset] {
	case 'i':
		var line int
		if line, e = buffer.AddrValue(ctx.addrs); e != nil {
			return
		}
		e = buffer.Insert(line, nbuf)
	case 'a':
		var line int
		if line, e = buffer.AddrValue(ctx.addrs); e != nil {
			return
		}
		e = buffer.Insert(line+1, nbuf)
	case 'c':
		var r [2]int
		if r, e = buffer.AddrRange(ctx.addrs); e != nil {
			return
		}
		buffer.Delete(r)
		e = buffer.Insert(r[0], nbuf)
	}
	return
}

var rxWrite = regexp.MustCompile("^(q)?(?: )?(!)?(.*)")

func cmdWrite(ctx *Context) (e error) {
	file := state.fileName
	quit := false
	run := false
	var r [2]int
	if ctx.cmdOffset == 0 {
		r[0] = 0
		r[1] = buffer.Len() - 1
	} else {
		if r, e = buffer.AddrRange(ctx.addrs); e != nil {
			return
		}
	}
	m := rxWrite.FindAllStringSubmatch(ctx.cmd[ctx.cmdOffset+1:], -1)
	if m[0][1] == "q" {
		quit = true
	}
	if m[0][2] == "!" {
		run = true
	}
	if len(m[0][3]) > 0 {
		file = m[0][3]
	}
	var lstr []string
	lstr, e = buffer.Get(r)
	if e != nil {
		return
	}
	if run {
		return fmt.Errorf("sending to stdin not yet supported")
	}
	var f *os.File
	oFlag := os.O_TRUNC

	if ctx.cmd[ctx.cmdOffset] == 'W' {
		oFlag = os.O_APPEND
	}
	if f, e = os.OpenFile(file, os.O_WRONLY|os.O_CREATE|oFlag, 0666); e != nil {
		return e
	}
	defer f.Close()
	for _, s := range lstr {
		_, e = fmt.Fprintf(f, "%s\n", s)
		if e != nil {
			return
		}
	}
	if quit {
		cmdQuit(ctx)
	}
	buffer.Clean()
	return
}

func cmdMark(ctx *Context) (e error) {
	if len(ctx.cmd)-1 <= ctx.cmdOffset {
		e = fmt.Errorf("no mark character supplied")
		return
	}
	c := ctx.cmd[ctx.cmdOffset+1]
	var l int
	if l, e = buffer.AddrValue(ctx.addrs); e != nil {
		return
	}
	e = buffer.SetMark(c, l)
	return
}

func cmdEdit(ctx *Context) (e error) {
	var addr int
	addr, e = buffer.AddrValue(ctx.addrs)
	if e != nil {
		return
	}
	// cmd or filename?
	cmd := ctx.cmd[ctx.cmdOffset]
	force := false
	if cmd == 'E' || cmd == 'r' {
		force = true
	} // else == 'e'
	if buffer.Dirty() && !force {
		return fmt.Errorf("warning: file modified")
	}
	filename := ctx.cmd[ctx.cmdOffset+1:]
	filename = filename[wsOffset(filename):]
	if filename[0] == '!' { // command, not filename
		// TODO
		return fmt.Errorf("command execution is not yet supported")
	} // filename
	if len(filename) == 0 {
		filename = state.fileName
	}
	// try to read in the file
	if _, e = os.Stat(filename); os.IsNotExist(e) && !*fSuppress {
		return fmt.Errorf("%s: No such file or directory", filename)
		// this is not fatal, we just start with an empty buffer
	}
	if cmd != 'r' {
		if buffer, e = FileToBuffer(filename); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
	} else {
		e = buffer.ReadFile(addr, filename)
		if len(state.fileName) == 0 {
			state.fileName = filename
		}
	}
	if !*fSuppress {
		fmt.Println(buffer.Size())
	}
	return
}

func cmdFile(ctx *Context) (e error) {
	newFile := ctx.cmd[ctx.cmdOffset:]
	newFile = newFile[wsOffset(newFile):]
	if len(newFile) > 0 {
		state.fileName = newFile
		return
	}
	fmt.Println(state.fileName)
	return
}

func cmdLine(ctx *Context) (e error) {
	addr, e := buffer.AddrValue(ctx.addrs)
	if e == nil {
		fmt.Println(addr + 1)
	}
	return
}

func cmdJoin(ctx *Context) (e error) {
	var r [2]int
	if r, e = buffer.AddrRangeOrLine(ctx.addrs); e != nil {
		return
	}
	// Technically only a range works, but a line isn't an error
	if r[0] == r[1] {
		return
	}

	joined := ""
	for l := r[0]; l <= r[1]; l++ {
		joined += buffer.GetMust(l, false)
	}
	if e = buffer.Delete(r); e != nil {
		return
	}
	e = buffer.Insert(r[0], []string{joined})
	return
}

func cmdMove(ctx *Context) (e error) {
	var r [2]int
	var dest int
	var lines []string
	if r, e = buffer.AddrRangeOrLine(ctx.addrs); e != nil {
		return
	}
	// must parse the destination
	destStr := ctx.cmd[ctx.cmdOffset+1:]
	var nctx Context
	if nctx.addrs, nctx.cmdOffset, e = buffer.ResolveAddrs(destStr); e != nil {
		return
	}
	// this is a bit hacky, but we're supposed to allow 0
	append := 1
	last := len(nctx.addrs) - 1
	if nctx.addrs[last] == -1 {
		nctx.addrs[last] = 0
		append = 0
	}
	if dest, e = buffer.AddrValue(nctx.addrs); e != nil {
		return
	}

	if lines, e = buffer.Get(r); e != nil {
		return
	}
	delt := r[1] - r[0] + 1
	if dest < r[0] {
		r[0] += delt
		r[1] += delt
	} else if dest > r[1] {
		//NOP
	} else {
		return fmt.Errorf("cannot move lines to within their own range")
	}

	// Should we throw an error if there's trailing stuff?
	if e = buffer.Insert(dest+append, lines); e != nil {
		return
	}
	e = buffer.Delete(r)
	return
}
