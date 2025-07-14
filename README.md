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
	var result map[string]any
	if err := Unmarshal([]byte(doc), &result); err != nil {
		panic(err)
	}

	fmt.Println(v)
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
	res, err := Marshal(stuff);
	if err != nil {
		panic(err)
	}

	fmt.Println(string(res))
}
```

MIT License
