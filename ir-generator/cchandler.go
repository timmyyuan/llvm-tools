package ir_generator

import (
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"log"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
)

type CCStyle int

const (
	CMakeStyle CCStyle = iota
	BearStyle
	AutoStyle
)

type CompilerCommand interface {
	SplitArgs() []string

	Run() error
	AddFlags(flags ...string)
	DropFlags(flags ...string)
	ReplaceCompiler(newcompiler string)
	ReplaceTargetExt(newext string)
	SwitchToO0()
	SwitchToC99()

	GetFile() string
	GetTarget() string
	GetDirectory() string
	String() string
}

type BuildOptions struct {
	Opt
	SolveHeaderNotFound bool
	SkipFailure         bool
	Incremental         bool
	Jobs                int
	Style               CCStyle
}

type CompilerDatabase struct {
	BuildOptions
	Commands []CompilerCommand
	TopDir   string

	taskMutex sync.Mutex
	failFiles map[string]bool
}

func (d *CompilerDatabase) GetAllExistTargets() []string {
	var result []string
	for _, c := range d.Commands {
		tgt := c.GetTarget()
		if _, err := os.Stat(tgt); os.IsNotExist(err) {
			continue
		}

		result = append(result, tgt)
	}
	return result
}

func (d *CompilerDatabase) LLVMLink(output string) error {
	linker := NewLLVMLinker()
	linker.Targets = d.GetAllExistTargets()
	linker.Output = output

	return linker.Link()
}

func (d *CompilerDatabase) EmitLLVM(clang string) {
	flags := []string{
		"-emit-llvm",
		"-g",
		"-Wno-shift-count-negative",
		"-Wno-division-by-zero",
		"-fno-inline-functions",
		"-Wno-ignored-optimization-argument",
		"-Xclang",
		"-disable-O0-optnone",
		//"-Xclang",
		//"-disable-llvm-optzns",
		"-flto",
		"-Xclang",
		"-disable-llvm-passes",
		"-Wno-everything",
	}

	for i := range d.Commands {
		d.Commands[i].ReplaceCompiler(clang)
		d.Commands[i].ReplaceTargetExt(".bc")
		// d.Commands[i].SwitchToO0()
		// d.Commands[i].SwitchToC99()
		d.Commands[i].AddFlags(flags...)
	}
}

func (d *CompilerDatabase) EmitClangAST(clang string) {
	flags := []string{
		"-emit-ast",
		"-g",
		"-Wno-shift-count-negative",
		"-Wno-division-by-zero",
		"-fno-inline-functions",
		"-Wno-ignored-optimization-argument",
		"-Xclang",
		"-disable-O0-optnone",
		"-Wno-everything",
	}

	for i := range d.Commands {
		d.Commands[i].ReplaceCompiler(clang)
		d.Commands[i].ReplaceTargetExt(".ast")
		// d.Commands[i].SwitchToO0()
		// d.Commands[i].SwitchToC99()
		d.Commands[i].AddFlags(flags...)
	}
}

func (d *CompilerDatabase) dumpStatus() {
	d.taskMutex.Lock()
	for k := range d.failFiles {
		fmt.Println("Failed", k)
	}
	failed := float32(len(d.failFiles))
	d.taskMutex.Unlock()

	total := float32(len(d.Commands))
	fmt.Printf("Compilation success rate: %.2f%%\n", (total-failed)/total*100)
}

func (d *CompilerDatabase) Dump() {
	b, err := json.MarshalIndent(d.Commands, "", "    ")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(string(b))
}

func (d *CompilerDatabase) markFailed(tgt string) {
	d.taskMutex.Lock()
	d.failFiles[tgt] = true
	d.taskMutex.Unlock()
}

func (d *CompilerDatabase) run(index int) {
	c := d.Commands[index]
	total := len(d.Commands)
	rate := int(math.Round(float64(index+1) / float64(total) * 100))

	tgt := c.GetTarget()
	directory := c.GetDirectory()

	if !d.needRun(c) {
		fmt.Printf("built    [%3d%%] %s\n", rate, tgt)
		return
	}

	color.Green("building [%3d%%] %s", rate, tgt)
	err := c.Run()
	if err != nil && !d.SkipFailure {
		args := c.SplitArgs()
		color.Red("Failed commands:")
		for _, a := range args {
			fmt.Println(a)
		}

		log.Println(c)
		log.Fatalln(err)
	}

	if err != nil {
		d.markFailed(tgt)
		return
	}

	fmt.Printf("built    [%3d%%] %s\n", rate, tgt)

	if !d.Opt.NeedRun() {
		return
	}

	// avoid data race
	opt := NewOpt(d.Opt)

	if err = opt.Run(tgt, directory); err != nil {
		d.markFailed(tgt)
		return
	}

	color.Cyan("opt      [%3d%%] %s", rate, tgt)
}

func (d *CompilerDatabase) needRun(c CompilerCommand) bool {
	if d.Incremental == false {
		return true
	}

	tgt := c.GetTarget()
	if info, err := os.Stat(tgt); err == nil {
		return info.Size() == 0
	}

	return true
}

func (d *CompilerDatabase) Run() {
	fmt.Println()
	total := len(d.Commands)
	for i := 0; i < total; i++ {
		d.run(i)
	}

	if d.SkipFailure {
		d.dumpStatus()
	}
}

func (d *CompilerDatabase) RunOnly(file string) {
	fmt.Println()
	for i := 0; i < len(d.Commands); i++ {
		cmd := d.Commands[i]
		if strings.HasSuffix(cmd.GetFile(), file) {
			fmt.Println(cmd)
			d.run(i)
		}
	}
}

func (d *CompilerDatabase) RunParallel() {
	taskCh := make(chan int, d.Jobs)
	total := len(d.Commands)

	var wg sync.WaitGroup
	wg.Add(total)
	for i := 0; i < d.Jobs; i++ {
		go func() {
			for task := range taskCh {
				d.run(task)
				wg.Done()
			}
		}()
	}

	fmt.Println()
	for i := 0; i < total; i++ {
		if !d.needRun(d.Commands[i]) {
			wg.Done()
			rate := int(math.Round(float64(i+1) / float64(total) * 100))
			fmt.Printf("built    [%3d%%] %s\n", rate, d.Commands[i].GetTarget())
			continue
		}

		taskCh <- i
	}

	close(taskCh)
	wg.Wait()

	if d.SkipFailure {
		d.dumpStatus()
	}
}

func (d *CompilerDatabase) Rewrite(ccjson string) {
	b, err := json.MarshalIndent(d, "", "    ")
	if err != nil {
		log.Fatalln(err)
	}

	err = os.WriteFile(ccjson, b, 0644)
	if err != nil {
		log.Fatalln(err)
	}
}

func (d *CompilerDatabase) Load(ccjson string) {
	b, err := os.ReadFile(ccjson)
	if err != nil {
		log.Fatalln(err)
	}

	var ok bool

	switch d.Style {
	case CMakeStyle:
		loadCommands[*CMakeCC](d, b, true)
	case BearStyle:
		loadCommands[*BearCC](d, b, true)
	case AutoStyle:
		if ok = loadCommands[*BearCC](d, b, false); ok {
			d.Style = BearStyle
			return
		}
		if ok = loadCommands[*CMakeCC](d, b, false); ok {
			d.Style = CMakeStyle
			return
		}
	}
}

func loadCommands[T CompilerCommand](d *CompilerDatabase, b []byte, reportFatal bool) bool {
	var commands []T
	fileSet := make(map[string]bool)
	if err := json.Unmarshal(b, &commands); err != nil {
		if !reportFatal {
			log.Fatalln(err)
		}
		return false
	}

	var hasEmpty bool
	for i := 0; i < len(commands) && !hasEmpty; i++ {
		if len(commands[i].SplitArgs()) == 0 {
			hasEmpty = true
		}
	}

	if hasEmpty {
		return false
	}

	isInvalidSource := func(file string) bool {
		suffix := []string{".s", ".S"}
		for _, s := range suffix {
			if strings.HasSuffix(file, s) {
				return true
			}
		}
		return false
	}

	for i := range commands {
		file := commands[i].GetTarget()
		if fileSet[file] {
			continue
		}

		if isInvalidSource(commands[i].GetFile()) {
			continue
		}

		d.Commands = append(d.Commands, commands[i])
		fileSet[file] = true
	}
	return len(commands) != 0
}

func NewCompilerDataBase() *CompilerDatabase {
	return &CompilerDatabase{
		BuildOptions: BuildOptions{Jobs: runtime.NumCPU() / 2},
		failFiles:    make(map[string]bool),
	}
}
