package main

import (
	"fernctl/internal/ssm"
	"flag"
	"fmt"
	"os"
)

func main() {

	flag.Parse()

	if len(flag.Args()) < 3 {
		fmt.Println("Not enough parameters")
		os.Exit(1)
	}

	cmd := flag.Arg(0)

	if cmd == "ssm" {

		s := ssm.NewService()
		err := s.Handle(flag.Args()[1:])
		if err != nil {
			fmt.Printf("error: %s\n", err)
		}
	}
}
