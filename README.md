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

### Struct Tags

The library supports struct tags for customizing field names and behavior, similar to `encoding/json`:

#### Field Renaming

Use the `huml` tag to specify a custom field name in the HUML output:

```go
type User struct {
    FirstName string `huml:"first_name"`
    LastName  string `huml:"last_name"`
    Age       int    `huml:"age"`
}
```

#### Skipping Fields

Use `huml:"-"` to skip a field during marshalling:

```go
type Config struct {
    PublicKey  string `huml:"public_key"`
    PrivateKey string `huml:"-"` // This field will be skipped
}
```

#### Omit Empty Values

Use the `omitempty` option to skip fields with zero/empty values:

```go
type Profile struct {
    Name     string  `huml:"name"`
    Email    string  `huml:"email,omitempty"`    // Omitted if empty
    Age      int     `huml:"age,omitempty"`     // Omitted if 0
    Bio      string  `huml:"bio,omitempty"`      // Omitted if empty
    Tags     []string `huml:"tags,omitempty"`     // Omitted if empty slice
    Metadata map[string]string `huml:"metadata,omitempty"` // Omitted if empty map
}
```

When marshalling, fields with `omitempty` are skipped if they are:
- Empty strings (`""`)
- Zero numeric values (`0`, `0.0`)
- `false` booleans
- `nil` pointers
- Empty slices, arrays, or maps
- Structs where all exported fields are empty

#### Complete Example

```go
package main

import (
    "fmt"
    "github.com/huml-lang/go-huml"
)

type Person struct {
    Name        string   `huml:"name"`
    Age         int      `huml:"age,omitempty"`
    Email       string   `huml:"email,omitempty"`
    SecretToken string   `huml:"-"` // Never marshalled
    Tags        []string `huml:"tags,omitempty"`
}

func main() {
    person := Person{
        Name:        "Alice",
        Age:         0,        // Will be omitted
        Email:       "",       // Will be omitted
        SecretToken: "secret", // Will be skipped
        Tags:        []string{"developer", "golang"},
    }

    data, _ := huml.Marshal(person)
    fmt.Println(string(data))
    // Output:
    // %HUML v0.1.0
    // name: "Alice"
    // tags::
    //   - "developer"
    //   - "golang"
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
