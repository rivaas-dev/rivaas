# Basic Example

This example demonstrates the most basic usage of Config package - loading configuration from a YAML file into a Go struct.

## Features Demonstrated

- **File Source**: Loading configuration from a YAML file
- **Struct Binding**: Mapping configuration to Go structs
- **Type Conversion**: Automatic conversion of different data types
- **Nested Structures**: Handling nested configuration objects
- **Arrays and Slices**: Loading string arrays and slices
- **Time Types**: Parsing time.Duration and time.Time values
- **URL Types**: Parsing URL strings into *url.URL

## Configuration Structure

The example includes various configuration types:

- **Basic Types**: string, int, bool, time.Duration
- **Complex Types**: time.Time, *url.URL
- **Collections**: []string (both as array and comma-separated string)
- **Nested Objects**: Worker configuration with timeout and address

## Running the Example

```bash
cd examples/basic
go run main.go
```

## Expected Output

```
{Foo:bar Timeout:10s Debug:true Worker:{Timeout:600 Address:http://localhost:8080} Date:2025-01-01 00:00:00 +0100 CET Roles:[admin user] Types:[x1 x2 x3] Types2:x1,x2,x3}
```

## Configuration File

The `config.yaml` file contains:

```yaml
foo: bar
timeout: 10s
debug: true
date: 2025-01-01T00:00:00+01:00
types: x1,x2,x3
roles:
  - admin
  - user
worker:
  timeout: 600
  address: http://localhost:8080
```

## Key Concepts

1. **Struct Tags**: Use `config:"field_name"` to map configuration keys to struct fields
2. **Type Safety**: Config package automatically converts values to the appropriate Go types
3. **Nested Mapping**: Use dot notation in struct tags for nested configuration
4. **Multiple Formats**: Arrays can be loaded from YAML arrays or comma-separated strings 