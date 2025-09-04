package main

import (
	"fmt"
	"log"

	"github.com/coderiantest/vingo"
)

func main() {
	data := map[string]interface{}{
		"user": map[string]interface{}{
			"IsAdmin":     true,
			"IsModerator": false,
		},
	}

	result, err := vingo.Render("./examples/index.html", data)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result)
}
