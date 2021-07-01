package main

import "fmt"

func main() {
	wtg := new(telegramWrap)
	if err := wtg.newClient(); err == nil {
		wtg.Run(func() {})
	} else {
		fmt.Println(err)
	}
}
