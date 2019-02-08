package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// A FileBuffer manages a file being edited
// Note: FileBuffer is 0-addressed lines, so off-by-one from what `ed` expects.
type FileBuffer struct {
	buffer []string // all lines we know about, they never get delited
	file   []int    // sequence of buffer lines
	dirty  bool     // tracks if the file has been modifed
	addr   int      // current file address
	marks  map[byte]int
}

// NewFileBuffer creats a new FileBuffer object
func NewFileBuffer(in []string) *FileBuffer {
	f := &FileBuffer{
		buffer: in,
		file:   []int{},
		dirty:  false,
		addr:   0,
		marks:  make(map[byte]int),
	}
	for i := range f.buffer {
		f.file = append(f.file, i)
	}
	return f
}

// ErrOOB line is out of bounds
var ErrOOB = fmt.Errorf("line is out of bounds")
var ErrINV = fmt.Errorf("invalid address")

// OOB checks if a line is out of bounds
func (f *FileBuffer) OOB(l int) bool {
	if l < 0 || l >= len(f.file) {
		return true
	}
	return false
}

func (f *FileBuffer) GetMust(line int, set bool) string {
	if set {
		f.addr = line
	}
	return f.buffer[f.file[line]]
}

func (f *FileBuffer) Get(r [2]int) (lines []string, e error) {
	if f.OOB(r[0]) || f.OOB(r[1]) {
		e = ErrOOB
		return
	}
	for l := r[0]; l <= r[1]; l++ {
		lines = append(lines, f.buffer[f.file[l]])
		f.addr = l
	}
	return
}

// Delete unmaps lines from the file
func (f *FileBuffer) Delete(r [2]int) (e error) {
	blines := []int{}
	for l := r[0]; l <= r[1]; l++ {
		if f.OOB(l) {
			return ErrOOB
		}
		blines = append(blines, f.file[l])
	}
	for _, b := range blines {
		for i, l := range f.file {
			if l == b {
				f.file = append(f.file[:i], f.file[i+1:]...)
				break
			}
		}
	}
	f.dirty = true
	f.addr = r[0] + 1
	if f.OOB(f.addr) {
		f.addr = 0
	}
	return
}

// Insert adds nlines to buffer and inserts them at line
func (f *FileBuffer) Insert(line int, nlines []string) (e error) {
	if f.OOB(line) {
		return ErrOOB
	}
	if len(nlines) == 0 {
		return
	}
	first := len(f.buffer)
	f.buffer = append(f.buffer, nlines...)
	nf := []int{}
	for i := first; i < len(f.buffer); i++ {
		nf = append(nf, i)
	}
	f.file = append(f.file[:line], append(nf, f.file[line:]...)...)
	f.dirty = true
	f.addr = line + len(nlines) - 1
	return
}

// Len returns the current file length
func (f *FileBuffer) Len() int {
	return len(f.file)
}

// Dirty returns whether the file has changed
func (f *FileBuffer) Dirty() bool {
	return f.dirty
}

// GetAddr gets the current file addr
func (f *FileBuffer) GetAddr() int {
	return f.addr
}

// SetAddr sets the current addr, errors if OOB
func (f *FileBuffer) SetAddr(i int) (e error) {
	if f.OOB(i) {
		return ErrOOB
	}
	f.addr = i
	return
}

// Clean resets the dirty flag
func (f *FileBuffer) Clean() {
	f.dirty = false
}

// FileToBuffer reads a file and creates a new FileBuffer from it
func FileToBuffer(file string) (fb *FileBuffer, e error) {
	b := []string{}
	f, e := os.Open(file)
	if e != nil {
		e = fmt.Errorf("could not read file: %v", e)
		return nil, nil
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		b = append(b, s.Text())
	}
	fb = NewFileBuffer(b)
	return
}

// flags
var (
	fSuppress = flag.Bool("s", false, "suppress counts")
	fPrompt   = flag.String("p", "*", "specify a command prompt")
)

var buffer *FileBuffer
var state struct {
	fileName string // current filename
	lastErr  error
	printErr bool
}

// re helpers
func reGroup(s string) string {
	return "(" + s + ")"
}

func reOr(s ...string) string {
	return strings.Join(s, "|")
}

func reOpt(s string) string {
	return s + "?"
}

func reStart(s string) string {
	return "^" + s
}

// addr regex strings
var (
	reWhitespace   = "(\\s)"
	reSingleSymbol = "([.$])"
	reNumber       = "([0-9]+)"
	reOffset       = reGroup("([-+])" + reOpt(reNumber))
	reMark         = "'([a-z])"
	reRE           = "(\\/((?:\\\\/|[^\\/])*)\\/|\\?((?:\\\\?|[^\\?])*)\\?)"
	reSingle       = reGroup(reOr(reSingleSymbol, reNumber, reOffset, reMark, reRE))
	reOff          = "(?:\\s+" + reGroup(reOr(reNumber, reOffset)) + ")"
)

var (
	rxWhitespace   = regexp.MustCompile(reStart(reWhitespace))
	rxSingleSymbol = regexp.MustCompile(reStart(reSingleSymbol))
	rxNumber       = regexp.MustCompile(reStart(reNumber))
	rxOffset       = regexp.MustCompile(reStart(reOffset))
	rxMark         = regexp.MustCompile(reStart(reMark))
	rxRE           = regexp.MustCompile(reStart(reRE))
	rxSingle       = regexp.MustCompile(reStart(reSingle))
	rxOff          = regexp.MustCompile(reStart(reOff))
)

// ResolveOffset resolves an offset to an addr
func (f *FileBuffer) ResolveOffset(cmd string) (offset, cmdOffset int, e error) {
	ms := rxOff.FindStringSubmatch(cmd)
	// 0: full
	// 1: without whitespace
	// 2: num
	// 3: offset full
	// 4: offset +/-
	// 5: offset num
	if len(ms) == 0 {
		return
	}
	n := 1
	cmdOffset = len(ms[0])
	switch {
	case len(ms[2]) > 0:
		// num
		if n, e = strconv.Atoi(ms[2]); e != nil {
			return
		}
		offset = n
	case len(ms[3]) > 0:
		// offset
		if len(ms[5]) > 0 {
			if n, e = strconv.Atoi(ms[5]); e != nil {
				return
			}
		}
		switch ms[4][0] {
		case '+':
			offset = n
		case '-':
			offset = -n
		}
	}
	return
}

// ResolveAddr resolves a command address from a cmd string
// - makes no attempt to verify that the resulting addr is valid
func (f *FileBuffer) ResolveAddr(cmd string) (line, cmdOffset int, e error) {
	line = f.GetAddr()
	cmdOffset = 0
	m := rxSingle.FindString(cmd)
	if len(m) == 0 {
		// no match
		return
	}
	cmdOffset = len(m)
	switch {
	case rxSingleSymbol.MatchString(m):
		// no need to rematch; these are all single char
		switch m[0] {
		case '.':
			// current
		case '$':
			// last
			line = f.Len() - 1
		}
	case rxNumber.MatchString(m):
		var n int
		ns := rxNumber.FindString(m)
		if n, e = strconv.Atoi(ns); e != nil {
			return
		}
		line = n - 1
	case rxOffset.MatchString(m):
		n := 1
		ns := rxOffset.FindStringSubmatch(m)
		if len(ns[3]) > 0 {
			if n, e = strconv.Atoi(ns[3]); e != nil {
				return
			}
		}
		switch ns[2][0] {
		case '+':
			line = f.GetAddr() + n
		case '-':
			line = f.GetAddr() - n
		}
	case rxMark.MatchString(m):
		c := m[1] // len should already be verified by regexp
		line, e = buffer.GetMark(c)
	case rxRE.MatchString(m):
		r := rxRE.FindAllStringSubmatch(m, -1)
		// 0: full
		// 1: regexp w/ delim
		// 2: regexp
		if len(r) < 1 || len(r[0]) < 3 {
			e = fmt.Errorf("invalid regexp: %s", m)
			return
		}
		restr := r[0][2]
		sign := 1
		if r[0][0][0] == '?' {
			sign = -1
			restr = r[0][3]
		}
		var re *regexp.Regexp
		if re, e = regexp.Compile(restr); e != nil {
			e = fmt.Errorf("invalid regexp: %v", e)
			return
		}
		var c int
		for i := 0; i < f.Len(); i++ {
			c = (sign*i + f.GetAddr() + f.Len()) % f.Len()
			if re.MatchString(f.GetMust(c, false)) {
				line = c
				return
			}
		}
		e = fmt.Errorf("regexp not found: %s", restr)
	}
	if e != nil {
		return
	}

	for {
		var off, cmdoff int
		if off, cmdoff, e = f.ResolveOffset(cmd[cmdOffset:]); e != nil {
			return
		}
		if cmdoff > 0 {
			// we got an offset
			line += off
			cmdOffset += cmdoff
		} else {
			// no more offsets
			break
		}
	}
	return
}

func wsOffset(cmd string) (o int) {
	o = 0
	ws := rxWhitespace.FindStringIndex(cmd)
	if ws != nil {
		o = ws[1]
	}
	return
}

// ResolveAddrs resolves all addrs at the begining of a line
// - makes no attempt to verify that the resulting addrs are valid
// - will always return at least one addr as long as there isn't an error
// - if an error is reached, return value behavior is undefined
func (f *FileBuffer) ResolveAddrs(cmd string) (lines []int, cmdOffset int, e error) {
	var line, off int

Loop:
	for cmdOffset < len(cmd) {
		cmdOffset += wsOffset(cmd[cmdOffset:])
		if line, off, e = f.ResolveAddr(cmd[cmdOffset:]); e != nil {
			return
		}
		lines = append(lines, line)
		cmdOffset += off
		cmdOffset += wsOffset(cmd[cmdOffset:])
		if len(cmd)-1 <= cmdOffset {
			return
		}
		switch cmd[cmdOffset] { // do we have more addrs?
		case ',':
			cmdOffset++
		case ';':
			// we're  the left side of a ; set the current addr
			if e = f.SetAddr(line); e != nil {
				return
			}
			cmdOffset++
		case '%':
			lines = append(lines, 0, f.Len()-1)
			cmdOffset++
			cmdOffset += wsOffset(cmd[cmdOffset:])
			return
		default:
			break Loop
		}
	}
	return
}

func (f *FileBuffer) AddrRange(addrs []int) (r [2]int, e error) {
	switch len(addrs) {
	case 0:
		e = ErrINV
		return
	case 1:
		r[0] = f.GetAddr()
		r[1] = addrs[0]
	default:
		r[0] = addrs[len(addrs)-2]
		r[1] = addrs[len(addrs)-1]
	}
	if f.OOB(r[0]) || f.OOB(r[1]) {
		e = ErrOOB
	}
	if r[0] > r[1] {
		e = fmt.Errorf("address out of order")
	}
	return
}

func (f *FileBuffer) AddrValue(addrs []int) (r int, e error) {
	if len(addrs) == 0 {
		e = ErrINV
		return
	}
	r = addrs[len(addrs)-1]
	if f.OOB(r) {
		e = ErrOOB
	}
	return
}

func (f *FileBuffer) SetMark(c byte, l int) (e error) {
	if f.OOB(l) {
		e = ErrOOB
		return
	}
	f.marks[c] = f.file[l]
	return
}

func (f *FileBuffer) GetMark(c byte) (l int, e error) {
	bl, ok := f.marks[c]
	if !ok {
		return -1, fmt.Errorf("no such mark: %c", c)
	}
	for i := 0; i < f.Len(); i++ {
		if f.file[i] == bl {
			l = i
			return
		}
	}
	return -1, fmt.Errorf("mark was cleared: %c", c)
}

// A Context is passed to an invoked command
type Context struct {
	cmd       string // full command string
	cmdOffset int    // start of the command after address resolution
	addrs     []int  // resolved addresses
}

// A Command can be run with a Context and returns an error
type Command func(*Context) error

func (f *FileBuffer) AddrRangeOrLine(addrs []int) (r [2]int, e error) {
	if len(addrs) > 1 {
		// delete a range
		if r, e = buffer.AddrRange(addrs); e != nil {
			return
		}
	} else {
		// delete a line
		if r[0], e = buffer.AddrValue(addrs); e != nil {
			return
		}
		r[1] = r[0]
	}
	return
}

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
		fmt.Printf("%s\n", buffer.GetMust(l, true))
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
		r[1] = buffer.Len()
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

var cmds = map[byte]Command{
	'q': cmdQuit,
	'Q': cmdQuit,
	'd': cmdDelete,
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
}

// parseInput
func run(cmd string) (e error) {
	ctx := &Context{
		cmd: cmd,
	}
	if ctx.addrs, ctx.cmdOffset, e = buffer.ResolveAddrs(cmd); e != nil {
		return
	}
	if len(cmd)-1 <= ctx.cmdOffset {
		// no command, default ot print
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

// main
func main() {
	var e error
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [-s] [-p <prompt>] [file]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) > 1 { // we only except one additional argument
		flag.Usage()
		os.Exit(1)
	}
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
