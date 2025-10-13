package huml_test

import (
	"fmt"

	"github.com/huml-lang/go-huml"
)

func ExampleUnmarshal() {
	doc := `
name: "Alice"
age: 30
active: true
`
	var result map[string]any
	if err := huml.Unmarshal([]byte(doc), &result); err != nil {
		panic(err)
	}

	fmt.Println(result["name"])
	fmt.Println(result["age"])
	fmt.Println(result["active"])
	// Output:
	// Alice
	// 30
	// true
}

func ExampleMarshal() {
	data := map[string]any{
		"name":   "Alice",
		"age":    30,
		"active": true,
	}

	res, err := huml.Marshal(data)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(res))
	// Output:
	// %HUML v0.1.0
	// active: true
	// age: 30
	// name: "Alice"
}
