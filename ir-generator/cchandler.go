package ir_generator

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type CompilerCommandCmake struct {
	Directory string `json:"directory"`
	Command   string `json:"command"`
	File      string `json:"file"`
}

func (c *CompilerCommandCmake) ReplaceCompiler(newcompiler string) {
	splits := strings.Split(c.Command, " ")
	splits[0] = newcompiler
	c.Command = strings.Join(splits, " ")
}

func (c *CompilerCommandCmake) ReplaceTargetExt(newext string) {
	splits := strings.Split(c.Command, " ")
	for i := 0; i < len(splits); i++ {
		if splits[i] != "-o" {
			continue
		}

		if i+1 >= len(splits) {
			log.Fatalln("There should be a valid filename behind `-o` flag")
		}

		filename := splits[i+1]
		filename = filename[:len(filename)-len(filepath.Ext(filename))]
		splits[i+1] = filename + newext
		break
	}

	c.Command = strings.Join(splits, " ")
}

func (c *CompilerCommandCmake) AddFlags(flags ...string) {
	splits := strings.Split(c.Command, " ")
	var index int
	for index = 0; index < len(splits); index += 1 {
		if splits[index] == "-c" {
			break
		}
	}
	var newsplits []string
	newsplits = append(newsplits, splits[:index]...)
	newsplits = append(newsplits, flags...)
	newsplits = append(newsplits, splits[index:]...)

	c.Command = strings.Join(newsplits, " ")
}

func (c *CompilerCommandCmake) SwitchToO0() {
	splits := strings.Split(c.Command, " ")
	olist := []string{
		"-O1",
		"-O2",
		"-O3",
		"-Os",
		"-Oz",
		"-Ofast",
	}

	for i := 0; i < len(splits); i++ {
		if !strings.HasPrefix(splits[i], "-O") {
			continue
		}

		for j := 0; j < len(olist); j++ {
			if olist[j] == splits[i] {
				splits[i] = "-O0"
			}
		}
	}

	c.Command = strings.Join(splits, " ")
}

func (c *CompilerCommandCmake) SwitchToC99() {
	splits := strings.Split(c.Command, " ")
	olist := []string{
		"-std=gnu99",
	}

	for i := 0; i < len(splits); i++ {
		if !strings.HasPrefix(splits[i], "-std=") {
			continue
		}

		for j := 0; j < len(olist); j++ {
			if olist[j] == splits[i] {
				splits[i] = "-std=c99"
			}
		}
	}

	c.Command = strings.Join(splits, " ")
}

func (c *CompilerCommandCmake) EscapeQuotes() {
	c.Command = strings.ReplaceAll(c.Command, "\\\"", "\"")
}

func (c *CompilerCommandCmake) GetLocalHeaders() []string {
	splits := strings.Split(c.Command, " ")
	var headers []string
	for i := 0; i < len(splits); i++ {
		if splits[i] == "-include" {
			headers = append(headers, splits[i+1])
		}
	}
	return headers
}

func (c *CompilerCommandCmake) Run() {
	args := strings.Split(c.Command, " ")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = c.Directory

	if err := cmd.Run(); err != nil {
		log.Println(c.Command)
		log.Fatalln(err)
	}
}

type CompilerDatabase struct {
	Commands []CompilerCommandCmake
	TopDir   string
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
		"-Wno-everything",
	}

	for i := range d.Commands {
		d.Commands[i].ReplaceCompiler(clang)
		d.Commands[i].ReplaceTargetExt(".bc")
		d.Commands[i].SwitchToO0()
		// d.Commands[i].SwitchToC99()
		d.Commands[i].AddFlags(flags...)
		d.Commands[i].EscapeQuotes()
	}
}

func (d *CompilerDatabase) Dump() {
	b, err := json.MarshalIndent(d.Commands, "", "    ")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(string(b))
}

func (d *CompilerDatabase) run(c CompilerCommandCmake) {
	headers := c.GetLocalHeaders()
	if len(headers) == 0 {
		c.Run()
		return
	}

	walkFn := func(path string, dir os.DirEntry, err error) error {
		base := filepath.Base(path)
		for i := 0; i < len(headers); i++ {
			if headers[i] == base {
			}
		}
		return nil
	}

	_ = filepath.WalkDir(d.TopDir, walkFn)

	c.Run()
}

func (d *CompilerDatabase) Run() {
	for i := 0; i < len(d.Commands); i++ {
		d.run(d.Commands[i])
	}
}

func (d *CompilerDatabase) RunParallel() {
	jobs := runtime.NumCPU() / 2
	taskCh := make(chan CompilerCommandCmake, jobs)

	for i := 0; i < jobs; i++ {
		go func() {
			for task := range taskCh {
				d.run(task)
			}
		}()
	}

	total := len(d.Commands)
	for i := 0; i < total; i++ {
		taskCh <- d.Commands[i]
		log.Printf("processing [%d/%d]\n", i, total)
	}

	close(taskCh)
}

func NewCompilerDataBase(ccjson string) *CompilerDatabase {
	b, err := os.ReadFile(ccjson)
	if err != nil {
		log.Fatalln(err)
	}

	d := &CompilerDatabase{}
	if err = json.Unmarshal(b, &d.Commands); err != nil {
		log.Fatalln(err)
	}

	return d
}
