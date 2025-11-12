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

func ExampleMarshal_structTags() {
	// Struct tags allow you to customize field names and behavior
	type Person struct {
		Name        string   `huml:"name"`
		Age         int      `huml:"age,omitempty"` // Omitted if zero
		Email       string   `huml:"email,omitempty"`
		SecretToken string   `huml:"-"` // Always skipped
		Tags        []string `huml:"tags,omitempty"`
	}

	person := Person{
		Name:        "Alice",
		Age:         0,        // Will be omitted
		Email:       "",       // Will be omitted
		SecretToken: "secret", // Will be skipped
		Tags:        []string{"developer", "golang"},
	}

	res, err := huml.Marshal(person)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(res))
	// Output:
	// %HUML v0.1.0
	// name: "Alice"
	// tags::
	//   - "developer"
	//   - "golang"
}

func ExampleUnmarshal_structTags() {
	// Struct tags work for unmarshalling too - they map HUML keys to struct fields
	type User struct {
		FirstName string `huml:"first_name"`
		LastName  string `huml:"last_name"`
		Age       int    `huml:"age"`
	}

	doc := `
first_name: "Alice"
last_name: "Smith"
age: 30
`

	var user User
	if err := huml.Unmarshal([]byte(doc), &user); err != nil {
		panic(err)
	}

	fmt.Printf("Name: %s %s, Age: %d\n", user.FirstName, user.LastName, user.Age)
	// Output:
	// Name: Alice Smith, Age: 30
}
