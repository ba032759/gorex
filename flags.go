package main

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

// rex's flags. Each command declares its flags once, as the flag package
// does (a pointer, the spellings, the default, the usage): the parser,
// the help, the completion and spec/reference/cli.md read the
// declarations. The parser is rex's own, not flag's: flags come before,
// between and after the arguments, "--" starts a command to run, and a
// number such as -5 is an argument.

// value is a flag's value, the shape of flag.Value: Set takes what the
// command line gives, String shows the value, before parsing the default.
type value interface {
	Set(string) error
	String() string
}

// flagDef is one flag of a command.
type flagDef struct {
	names    []string // its spellings, without dashes: "c", "cwd", "d", "dir"
	usage    string   // its help; a `WORD` in it names the value
	val      value
	def      string // the default, as text
	isBool   bool   // takes no value: --name, or --name=false
	ignored  bool   // taken so that Rex's command lines run; it does nothing
	computed bool   // def says what the command computes, not a value
}

// name is the flag's last spelling, which names it in messages.
func (d *flagDef) name() string { return dashed(d.names[len(d.names)-1]) }

// spellings are the flag's spellings with their dashes: "-c, --cwd".
func (d *flagDef) spellings() string {
	var s []string
	for _, n := range d.names {
		s = append(s, dashed(n))
	}
	return strings.Join(s, ", ")
}

// placeholder and text split the usage into the name of the value and
// the help, as flag.UnquoteUsage does: "the `DIR` it starts in".
func (d *flagDef) placeholder() string {
	if _, after, ok := strings.Cut(d.usage, "`"); ok {
		name, _, _ := strings.Cut(after, "`")
		return name
	}
	if d.isBool {
		return ""
	}
	return "VALUE"
}

func (d *flagDef) text() string { return strings.ReplaceAll(d.usage, "`", "") }

// values says which values the flag takes, when its kind limits them.
func (d *flagDef) values() string {
	switch v := d.val.(type) {
	case *percentValue:
		return v.r.String()
	case *positionValue:
		return "from 1"
	case *durationValue:
		return "a duration, as 300ms, 10s, 2h45m"
	case *listValue:
		return "again for more"
	case *choiceValue:
		return strings.Join(v.values, " or ")
	}
	return ""
}

// ignore marks a flag that is taken and does nothing.
func (d *flagDef) ignore() *flagDef { d.ignored = true; return d }

// defaults says the default of a flag whose default the command computes.
func (d *flagDef) defaults(text string) *flagDef { d.def, d.computed = text, true; return d }

func dashed(s string) string {
	if len(s) == 1 {
		return "-" + s
	}
	return "--" + s
}

// flagSet is the flags of a command, in the order they were declared.
type flagSet struct {
	flags []*flagDef
	names map[string]*flagDef
	word  string // "flag", or "option" for the global options
}

func newFlagSet(word string) *flagSet { return &flagSet{names: map[string]*flagDef{}, word: word} }

func (f *flagSet) add(names, usage string, v value, isBool bool) *flagDef {
	d := &flagDef{names: strings.Split(names, ","), usage: usage, val: v, def: v.String(), isBool: isBool}
	for _, n := range d.names {
		if f.names[n] != nil {
			panic("rex: flag " + n + " declared twice")
		}
		f.names[n] = d
	}
	f.flags = append(f.flags, d)
	return d
}

// str declares a flag of text; empty is not given.
func (f *flagSet) str(p *string, names, def, usage string) *flagDef {
	*p = def
	return f.add(names, usage, (*strValue)(p), false)
}

// boolean declares a flag that is on or off.
func (f *flagSet) boolean(p *bool, names string, def bool, usage string) *flagDef {
	*p = def
	return f.add(names, usage, &boolValue{p, def}, true)
}

// duration declares a flag of a duration, as time.ParseDuration reads it.
func (f *flagSet) duration(p *time.Duration, names string, def time.Duration, usage string) *flagDef {
	*p = def
	return f.add(names, usage, &durationValue{p, def}, false)
}

// percent declares a flag of a percentage, with or without "%", in r.
func (f *flagSet) percent(p *float64, names string, def float64, r pctRange, usage string) *flagDef {
	*p = def
	return f.add(names, usage, &percentValue{p, def, r}, false)
}

// position declares a flag of a position from 1; 0 is not given.
func (f *flagSet) position(p *int, names, usage string) *flagDef {
	*p = 0
	return f.add(names, usage, (*positionValue)(p), false)
}

// choice declares a flag of text that takes one of values.
func (f *flagSet) choice(p *string, names, def string, values []string, usage string) *flagDef {
	*p = def
	return f.add(names, usage, &choiceValue{p, def, values}, false)
}

// list declares a flag that may repeat, each value added to the list.
func (f *flagSet) list(p *[]string, names, usage string) *flagDef {
	return f.add(names, usage, (*listValue)(p), false)
}

// call is what a command gets besides its flags.
type call struct {
	args    []string // the arguments that are not flags
	command []string // what follows "--"
	dashed  bool     // "--" was given
}

// parse reads a command's flags before, between and after its arguments.
func (f *flagSet) parse(args []string) (*call, error) {
	c := &call{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			c.command, c.dashed = args[i+1:], true
			break
		}
		if len(arg) < 2 || arg[0] != '-' || isNumber(arg) {
			c.args = append(c.args, arg)
			continue
		}
		var err error
		if i, err = f.one(args, i); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// parseLeading reads the flags before the first argument, as the global
// options before the command, and returns the rest.
func (f *flagSet) parseLeading(args []string) ([]string, error) {
	i := 0
	for ; i < len(args) && strings.HasPrefix(args[i], "-") && args[i] != "--help" && args[i] != "-h"; i++ {
		var err error
		if i, err = f.one(args, i); err != nil {
			return nil, err
		}
	}
	return args[i:], nil
}

// one reads the flag at args[i], and its value, and returns the index of
// the last word it read.
func (f *flagSet) one(args []string, i int) (int, error) {
	arg := args[i]
	name, val, hasVal := strings.Cut(strings.TrimLeft(arg, "-"), "=")
	d := f.names[name]
	if d == nil {
		return i, usagef("unknown %s %s", f.word, arg)
	}
	who := d.name()
	switch {
	case d.isBool:
		who = arg
		if !hasVal {
			val = "true"
		}
	case !hasVal:
		if i+1 >= len(args) {
			return i, usagef("%s needs a value", arg)
		}
		i++
		val = args[i]
	}
	err := d.val.Set(val)
	var ce *cliError
	var pe phraseError
	switch {
	case err == nil:
		return i, nil
	case errors.As(err, &ce):
		return i, err
	case errors.As(err, &pe):
		return i, usagef("%s %s", who, pe)
	}
	return i, usagef("%s: %v", who, err)
}

// phraseError is what a value says of itself: "takes a percentage".
type phraseError string

func (e phraseError) Error() string { return string(e) }

func isNumber(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// The kinds of values.

type strValue string

func (v *strValue) Set(s string) error { *v = strValue(s); return nil }
func (v *strValue) String() string     { return string(*v) }

// boolValue is a boolean flag's value; an empty one, as --focus=, is the
// default, as for every other flag.
type boolValue struct {
	p   *bool
	def bool
}

func (v *boolValue) Set(s string) error {
	switch s {
	case "":
		*v.p = v.def
	case "true", "1":
		*v.p = true
	case "false", "0":
		*v.p = false
	default:
		return phraseError("takes true or false")
	}
	return nil
}

func (v *boolValue) String() string { return strconv.FormatBool(*v.p) }

type durationValue struct {
	p   *time.Duration
	def time.Duration
}

func (v *durationValue) Set(s string) error {
	if s == "" {
		*v.p = v.def
		return nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*v.p = d
	return nil
}

func (v *durationValue) String() string {
	if *v.p == 0 {
		return ""
	}
	return v.p.String()
}

// pctRange is the percentages a flag takes: above lo and below hi, or,
// closed, from lo to hi.
type pctRange struct {
	lo, hi float64
	closed bool
}

// has tells whether n is in the range; NaN is in none.
func (r pctRange) has(n float64) bool {
	if r.closed {
		return n >= r.lo && n <= r.hi
	}
	return n > r.lo && n < r.hi
}

func (r pctRange) String() string {
	switch {
	case math.IsInf(r.hi, 1):
		return "above " + fmtFloat(r.lo)
	case r.closed:
		return "from " + fmtFloat(r.lo) + " to " + fmtFloat(r.hi)
	}
	return "above " + fmtFloat(r.lo) + " and below " + fmtFloat(r.hi)
}

type percentValue struct {
	p   *float64
	def float64
	r   pctRange
}

func (v *percentValue) Set(s string) error {
	if s == "" {
		*v.p = v.def
		return nil
	}
	n, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
	if err != nil || !v.r.has(n) {
		return phraseError("takes a percentage " + v.r.String())
	}
	*v.p = n
	return nil
}

func (v *percentValue) String() string {
	if *v.p == 0 {
		return ""
	}
	return fmtFloat(*v.p)
}

func fmtFloat(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

type positionValue int

func (v *positionValue) Set(s string) error {
	if s == "" {
		*v = 0
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return phraseError("takes a position from 1")
	}
	*v = positionValue(n)
	return nil
}

func (v *positionValue) String() string {
	if *v == 0 {
		return ""
	}
	return strconv.Itoa(int(*v))
}

type choiceValue struct {
	p      *string
	def    string
	values []string
}

func (v *choiceValue) Set(s string) error {
	switch {
	case s == "":
		*v.p = v.def
	case slices.Contains(v.values, s):
		*v.p = s
	default:
		return phraseError("takes " + strings.Join(v.values, " or "))
	}
	return nil
}

func (v *choiceValue) String() string { return *v.p }

type listValue []string

func (v *listValue) Set(s string) error { *v = append(*v, s); return nil }
func (v *listValue) String() string     { return strings.Join(*v, ",") }

// funcValue is a value whose Set is a function, as flag.Func.
type funcValue func(string) error

func (v funcValue) Set(s string) error { return v(s) }
func (v funcValue) String() string     { return "" }
