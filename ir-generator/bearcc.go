package ir_generator

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type BearCC struct {
	Directory string   `json:"directory"`
	Args      []string `json:"arguments"`
	File      string   `json:"file"`
}

func (c *BearCC) String() string {
	return strings.Join(c.Args, " ")
}

func (c *BearCC) SplitArgs() []string {
	return c.Args
}

func (c *BearCC) GetFile() string {
	return c.File
}

func (c *BearCC) GetTarget() string {
	splits := c.Args
	for i := 0; i < len(splits); i++ {
		if splits[i] != "-o" {
			continue
		}

		if i+1 >= len(splits) {
			log.Fatalln("There should be a valid filename behind `-o` flag")
		}

		return splits[i+1]
	}

	// if there is no `-o` flag
	return c.File + ".o"
}

func (c *BearCC) ReplaceCompiler(newcompiler string) {
	c.Args[0] = newcompiler
}

func (c *BearCC) ReplaceTargetExt(newext string) {
	splits := c.Args
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
		return
	}

	// if there is no `-o` flag
	c.Args = append(c.Args, "-o", c.Args[len(c.Args)-1]+newext)
}

func (c *BearCC) AddFlags(flags ...string) {
	splits := c.Args
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

	c.Args = newsplits
}

func (c *BearCC) DropFlags(flags ...string) {
	fmap := make(map[string]bool)
	for _, f := range flags {
		fmap[f] = true
	}

	splits := c.Args

	var newsplits []string
	for i := 0; i < len(splits); i++ {
		if _, ok := fmap[splits[i]]; !ok {
			newsplits = append(newsplits, splits[i])
		}
	}

	c.Args = newsplits
}

func (c *BearCC) SwitchToO0() {
	splits := c.Args
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
}

func (c *BearCC) SwitchToC99() {
	splits := c.Args
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
}

func (c *BearCC) Run() error {
	cmd := exec.Command(c.Args[0], c.Args[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = c.Directory

	return cmd.Run()
}
