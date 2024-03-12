package main

import (
	"io"
	"os"

	"github.com/tejasdeepakmasne/nesemu-go/hardware"
)

func main() {
	//progArgs := os.Args
	file, err := os.Open("./hardware/nestest.nes")
	if err != nil {
		panic(err)
	}

	contents, err := io.ReadAll(file)
	if err != nil {
		panic(err)
	}

	cpu := hardware.NewCPU()
	cpu.Load_and_interpret(contents)
}
