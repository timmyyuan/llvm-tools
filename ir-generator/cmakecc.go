package ir_generator

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type CMakeCC struct {
	Directory string `json:"directory"`
	Command   string `json:"command"`
	File      string `json:"file"`
}

func (c *CMakeCC) String() string {
	return c.Command
}

func (c *CMakeCC) SplitArgs() []string {
	var args []string

	splits := strings.Split(c.String(), " ")
	for _, s := range splits {
		if len(strings.TrimSpace(s)) != 0 {
			args = append(args, s)
		}
	}
	return args
}

func (c *CMakeCC) GetFile() string {
	return c.File
}

func (c *CMakeCC) GetDirectory() string {
	return c.Directory
}

func (c *CMakeCC) GetTarget() string {
	splits := strings.Split(c.Command, " ")
	for i := 0; i < len(splits); i++ {
		if splits[i] != "-o" {
			continue
		}

		if i+1 >= len(splits) {
			log.Fatalln("There should be a valid filename behind `-o` flag")
		}

		return splits[i+1]
	}

	ext := filepath.Ext(c.File)
	return c.File[:len(c.File)-len(ext)] + ".o"
}

func (c *CMakeCC) ReplaceCompiler(newcompiler string) {
	splits := strings.Split(c.Command, " ")
	splits[0] = newcompiler
	c.Command = strings.Join(splits, " ")
}

func (c *CMakeCC) ReplaceTargetExt(newext string) {
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

func (c *CMakeCC) AddFlags(flags ...string) {
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

func (c *CMakeCC) DropFlags(flags ...string) {
	fmap := make(map[string]bool)
	for _, f := range flags {
		fmap[f] = true
	}

	splits := strings.Split(c.Command, " ")

	var newsplits []string
	for i := 0; i < len(splits); i++ {
		if _, ok := fmap[splits[i]]; !ok {
			newsplits = append(newsplits, splits[i])
		}
	}

	c.Command = strings.Join(newsplits, " ")
}

func (c *CMakeCC) SwitchToO0() {
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

func (c *CMakeCC) SwitchToC99() {
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

func (c *CMakeCC) Run() error {
	cmd := exec.Command("sh", "-c", c.Command)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = c.Directory

	return cmd.Run()
}
