# semver for golang [![Build Status](https://travis-ci.org/blang/semver.svg?branch=master)](https://travis-ci.org/blang/semver) [![GoDoc](https://godoc.org/github.com/blang/semver/v4?status.svg)](https://godoc.org/github.com/blang/semver/v4) [![Coverage Status](https://img.shields.io/coveralls/blang/semver.svg)](https://coveralls.io/r/blang/semver?branch=master) [![Go Report Card](https://goreportcard.com/badge/github.com/blang/semver)](https://goreportcard.com/report/github.com/blang/semver)

semver is a [Semantic Versioning](http://semver.org/) library written in golang. It fully covers spec version `2.0.0`.

This fork adds the following features from the original:

- `^1.0.0` caret ranges (often used with npm)
- `~1.0.0` tilda ranges (often used with npm)
- `~>1.0.0` "stabby arrow" ranges (often used with Ruby)
- `1` -> `1.0.0`
- `2 - 4` -> `>=2.0.0`,`<4.0.0`

I have also updated the benchmarks at the bottom.

## Versioning

Old v1-v3 versions exist in the root of the repository for compatiblity reasons and will only receive bug fixes.

The current stable version is [_v4_](v4/) and is fully go-mod compatible.

## Usage

```bash
$ go get github.com/blang/semver/v4
# Or use fixed versions
$ go get github.com/blang/semver/v4@v4.0.0
```

Note: Always vendor your dependencies or fix on a specific version tag.

```go
import github.com/blang/semver/v4
v1, err := semver.Make("1.0.0-beta")
v2, err := semver.Make("2.0.0-beta")
v1.Compare(v2)
```

Also check the [GoDocs](http://godoc.org/github.com/blang/semver/v4).

## Why should I use this lib?

- Fully spec compatible
- No reflection
- No regex
- Fully tested (Coverage >99%)
- Readable parsing/validation errors
- Fast (See [Benchmarks](#benchmarks))
- Only Stdlib
- Uses values instead of pointers
- Many features, see below

## Features

- Parsing and validation at all levels
- Comparator-like comparisons
- Compare Helper Methods
- InPlace manipulation
- Ranges `>=1.0.0 <2.0.0 || >=3.0.0 !3.0.1-beta.1`
- Wildcards `>=1.x`, `<=2.5.x`
- Sortable (implements sort.Interface)
- database/sql compatible (sql.Scanner/Valuer)
- encoding/json compatible (json.Marshaler/Unmarshaler)

## Ranges

A `Range` is a set of conditions which specify which versions satisfy the range.

A condition is composed of an operator and a version. The supported operators are:

- `<1.0.0` Less than `1.0.0`
- `<=1.0.0` Less than or equal to `1.0.0`
- `>1.0.0` Greater than `1.0.0`
- `>=1.0.0` Greater than or equal to `1.0.0`
- `1.0.0`, `=1.0.0`, `==1.0.0` Equal to `1.0.0`
- `!1.0.0`, `!=1.0.0` Not equal to `1.0.0`. Excludes version `1.0.0`.

Note that spaces between the operator and the version will be gracefully tolerated.

A `Range` can link multiple `Ranges` separated by space:

Ranges can be linked by logical AND:

- `>1.0.0 <2.0.0` would match between both ranges, so `1.1.1` and `1.8.7` but not `1.0.0` or `2.0.0`
- `>1.0.0 <3.0.0 !2.0.3-beta.2` would match every version between `1.0.0` and `3.0.0` except `2.0.3-beta.2`

Ranges can also be linked by logical OR:

- `<2.0.0 || >=3.0.0` would match `1.x.x` and `3.x.x` but not `2.x.x`

AND has a higher precedence than OR. It's not possible to use brackets.

Ranges can be combined by both AND and OR

- `>1.0.0 <2.0.0 || >3.0.0 !4.2.1` would match `1.2.3`, `1.9.9`, `3.1.1`, but not `4.2.1`, `2.1.1`

Range usage:

```
v, err := semver.Parse("1.2.3")
expectedRange, err := semver.ParseRange(">1.0.0 <2.0.0 || >=3.0.0")
if expectedRange(v) {
    //valid
}

```

## Example

Have a look at full examples in [v4/examples/main.go](v4/examples/main.go)

```go
import github.com/blang/semver/v4

v, err := semver.Make("0.0.1-alpha.preview+123.github")
fmt.Printf("Major: %d\n", v.Major)
fmt.Printf("Minor: %d\n", v.Minor)
fmt.Printf("Patch: %d\n", v.Patch)
fmt.Printf("Pre: %s\n", v.Pre)
fmt.Printf("Build: %s\n", v.Build)

// Prerelease versions array
if len(v.Pre) > 0 {
    fmt.Println("Prerelease versions:")
    for i, pre := range v.Pre {
        fmt.Printf("%d: %q\n", i, pre)
    }
}

// Build meta data array
if len(v.Build) > 0 {
    fmt.Println("Build meta data:")
    for i, build := range v.Build {
        fmt.Printf("%d: %q\n", i, build)
    }
}

v001, err := semver.Make("0.0.1")
// Compare using helpers: v.GT(v2), v.LT, v.GTE, v.LTE
v001.GT(v) == true
v.LT(v001) == true
v.GTE(v) == true
v.LTE(v) == true

// Or use v.Compare(v2) for comparisons (-1, 0, 1):
v001.Compare(v) == 1
v.Compare(v001) == -1
v.Compare(v) == 0

// Manipulate Version in place:
v.Pre[0], err = semver.NewPRVersion("beta")
if err != nil {
    fmt.Printf("Error parsing pre release version: %q", err)
}

fmt.Println("\nValidate versions:")
v.Build[0] = "?"

err = v.Validate()
if err != nil {
    fmt.Printf("Validation failed: %s\n", err)
}
```

## Benchmarks

This fork is about 8% slower than the original:

```
goos: darwin
goarch: amd64
pkg: github.com/Jarred-Sumner/semver/v4
cpu: Intel(R) Core(TM) i9-9980HK CPU @ 2.40GHz
BenchmarkRangeParseSimple-16        	 2402451	       493.4 ns/op	     224 B/op	       7 allocs/op
BenchmarkRangeParseAverage-16       	 1235457	       967.9 ns/op	     456 B/op	      13 allocs/op
BenchmarkRangeParseComplex-16       	  392412	      3129 ns/op	    1736 B/op	      39 allocs/op
BenchmarkRangeMatchSimple-16        	100000000	        11.32 ns/op	       0 B/op	       0 allocs/op
BenchmarkRangeMatchAverage-16       	48316423	        25.28 ns/op	       0 B/op	       0 allocs/op
BenchmarkRangeMatchComplex-16       	17318359	        70.28 ns/op	       0 B/op	       0 allocs/op
BenchmarkRangeMatchNPM-16           	27442455	        43.70 ns/op	       0 B/op	       0 allocs/op
BenchmarkParseSimple-16             	 8453367	       139.5 ns/op	      48 B/op	       1 allocs/op
BenchmarkParseComplex-16            	 1795279	       687.0 ns/op	     256 B/op	       7 allocs/op
BenchmarkParseAverage-16            	 2672629	       466.3 ns/op	     163 B/op	       4 allocs/op
BenchmarkParseTolerantAverage-16    	 3093957	       394.1 ns/op	     132 B/op	       4 allocs/op
BenchmarkStringSimple-16            	40966444	        30.76 ns/op	       5 B/op	       1 allocs/op
BenchmarkStringLarger-16            	17750650	        65.63 ns/op	      32 B/op	       2 allocs/op
BenchmarkStringComplex-16           	12103804	        98.85 ns/op	      80 B/op	       3 allocs/op
BenchmarkStringAverage-16           	13836576	        89.55 ns/op	      45 B/op	       2 allocs/op
BenchmarkValidateSimple-16          	441872518	         2.589 ns/op	       0 B/op	   0 allocs/op
BenchmarkValidateComplex-16         	 7224438	       151.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkValidateAverage-16         	13011160	        88.88 ns/op	       0 B/op	       0 allocs/op
BenchmarkCompareSimple-16           	309173464	         3.999 ns/op	       0 B/op	   0 allocs/op
BenchmarkCompareComplex-16          	82620462	        12.37 ns/op	       0 B/op	       0 allocs/op
BenchmarkCompareAverage-16          	68264864	        17.24 ns/op	       0 B/op	       0 allocs/op
BenchmarkSort-16                    	 7555494	       160.2 ns/op	     248 B/op	       2 allocs/op
PASS
ok  	github.com/Jarred-Sumner/semver/v4	31.099s
```

Original:

```
goos: darwin
goarch: amd64
pkg: github.com/blang/semver/v4
cpu: Intel(R) Core(TM) i9-9980HK CPU @ 2.40GHz
BenchmarkRangeParseSimple-16        	 2580975	       453.4 ns/op	     224 B/op	       7 allocs/op
BenchmarkRangeParseAverage-16       	 1339657	       920.7 ns/op	     456 B/op	      13 allocs/op
BenchmarkRangeParseComplex-16       	  407686	      2845 ns/op	    1736 B/op	      39 allocs/op
BenchmarkRangeMatchSimple-16        	100000000	        11.10 ns/op	       0 B/op	       0 allocs/op
BenchmarkRangeMatchAverage-16       	44137429	        24.07 ns/op	       0 B/op	       0 allocs/op
BenchmarkRangeMatchComplex-16       	17746507	        66.63 ns/op	       0 B/op	       0 allocs/op
BenchmarkParseSimple-16             	 9203169	       129.8 ns/op	      48 B/op	       1 allocs/op
BenchmarkParseComplex-16            	 1875487	       624.6 ns/op	     256 B/op	       7 allocs/op
BenchmarkParseAverage-16            	 2838685	       426.4 ns/op	     163 B/op	       4 allocs/op
BenchmarkParseTolerantAverage-16    	 3281038	       375.9 ns/op	     132 B/op	       4 allocs/op
BenchmarkStringSimple-16            	40971831	        28.91 ns/op	       5 B/op	       1 allocs/op
BenchmarkStringLarger-16            	18104654	        65.11 ns/op	      32 B/op	       2 allocs/op
BenchmarkStringComplex-16           	11883522	        97.92 ns/op	      80 B/op	       3 allocs/op
BenchmarkStringAverage-16           	13932410	        87.39 ns/op	      45 B/op	       2 allocs/op
BenchmarkValidateSimple-16          	473300133	         2.546 ns/op	       0 B/op	       0 allocs/op
BenchmarkValidateComplex-16         	 8035316	       148.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkValidateAverage-16         	13491339	        88.06 ns/op	       0 B/op	       0 allocs/op
BenchmarkCompareSimple-16           	310534528	         3.861 ns/op	       0 B/op	       0 allocs/op
BenchmarkCompareComplex-16          	84087928	        12.82 ns/op	       0 B/op	       0 allocs/op
BenchmarkCompareAverage-16          	67448468	        16.62 ns/op	       0 B/op	       0 allocs/op
BenchmarkSort-16                    	 7902789	       156.0 ns/op	     248 B/op	       2 allocs/op
PASS
ok  	github.com/blang/semver/v4	29.348s
```

See benchmark cases at [semver_test.go](semver_test.go)

## Motivation

I simply couldn't find any lib supporting the full spec. Others were just wrong or used reflection and regex which i don't like.

## Contribution

Feel free to make a pull request. For bigger changes create a issue first to discuss about it.

## License

See [LICENSE](LICENSE) file.
