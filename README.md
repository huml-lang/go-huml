# go-huml

This is an experimental Go parser implementation for HUML (Human-oriented Markup Language). The API is similar to `encoding/json`.

[![Go Reference](https://pkg.go.dev/badge/github.com/huml-lang/go-huml.svg)](https://pkg.go.dev/github.com/huml-lang/go-huml)

## Usage

### Unmarshalling

```go
package main

import (
    "fmt"

    "github.com/huml-lang/go-huml"
)

func main() {
    doc := `
name: "Alice"
age: 30
active: true
`
    var result map[string]any
    if err := huml.Unmarshal([]byte(doc), &result); err != nil {
        panic(err)
    }

    fmt.Println(result["name"])   // Alice
    fmt.Println(result["age"])    // 30
    fmt.Println(result["active"]) // true
}
```

### Marshalling

```go
package main

import (
    "fmt"

    "github.com/huml-lang/go-huml"
)

func main() {
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
```

See the [package documentation](https://pkg.go.dev/github.com/huml-lang/go-huml) for more examples and API reference.

## Development Setup

This project uses git submodules for test data. After cloning the repository, initialize the submodules:

```bash
git submodule update --init --recursive
```

This will pull the test cases from the `huml-lang/tests` repository into the `tests/` directory.

MIT License
