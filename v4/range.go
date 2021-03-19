package semver

import (
	"fmt"
	"strconv"
	"strings"
)

type wildcardType int

const (
	noneWildcard  wildcardType = iota
	majorWildcard wildcardType = 1
	minorWildcard wildcardType = 2
	patchWildcard wildcardType = 3
)

func wildcardTypefromInt(i int) wildcardType {
	switch i {
	case 1:
		return majorWildcard
	case 2:
		return minorWildcard
	case 3:
		return patchWildcard
	default:
		return noneWildcard
	}
}

type comparator func(Version, Version) bool

var (
	compEQ comparator = func(v1 Version, v2 Version) bool {
		return v1.Compare(v2) == 0
	}
	compNE = func(v1 Version, v2 Version) bool {
		return v1.Compare(v2) != 0
	}
	compGT = func(v1 Version, v2 Version) bool {
		return v1.Compare(v2) == 1
	}
	compGE = func(v1 Version, v2 Version) bool {
		return v1.Compare(v2) >= 0
	}
	compLT = func(v1 Version, v2 Version) bool {
		return v1.Compare(v2) == -1
	}
	compLE = func(v1 Version, v2 Version) bool {
		return v1.Compare(v2) <= 0
	}
)

type versionRange struct {
	v Version
	c comparator
}

// rangeFunc creates a Range from the given versionRange.
func (vr *versionRange) rangeFunc() Range {
	return Range(func(v Version) bool {
		return vr.c(v, vr.v)
	})
}

// Range represents a range of versions.
// A Range can be used to check if a Version satisfies it:
//
//     range, err := semver.ParseRange(">1.0.0 <2.0.0")
//     range(semver.MustParse("1.1.1") // returns true
type Range func(Version) bool

// OR combines the existing Range with another Range using logical OR.
func (rf Range) OR(f Range) Range {
	return Range(func(v Version) bool {
		return rf(v) || f(v)
	})
}

// AND combines the existing Range with another Range using logical AND.
func (rf Range) AND(f Range) Range {
	return Range(func(v Version) bool {
		return rf(v) && f(v)
	})
}

// ParseRange parses a range and returns a Range.
// If the range could not be parsed an error is returned.
//
// Valid ranges are:
//   - "<1.0.0"
//   - "<=1.0.0"
//   - ">1.0.0"
//   - ">=1.0.0"
//   - "1.0.0", "=1.0.0", "==1.0.0"
//   - "!1.0.0", "!=1.0.0"
//
// A Range can consist of multiple ranges separated by space:
// Ranges can be linked by logical AND:
//   - ">1.0.0 <2.0.0" would match between both ranges, so "1.1.1" and "1.8.7" but not "1.0.0" or "2.0.0"
//   - ">1.0.0 <3.0.0 !2.0.3-beta.2" would match every version between 1.0.0 and 3.0.0 except 2.0.3-beta.2
//
// Ranges can also be linked by logical OR:
//   - "<2.0.0 || >=3.0.0" would match "1.x.x" and "3.x.x" but not "2.x.x"
//
// AND has a higher precedence than OR. It's not possible to use brackets.
//
// Ranges can be combined by both AND and OR
//
//  - `>1.0.0 <2.0.0 || >3.0.0 !4.2.1` would match `1.2.3`, `1.9.9`, `3.1.1`, but not `4.2.1`, `2.1.1`
func ParseRange(s string) (Range, error) {
	parts := splitAndTrim(s)
	orParts, err := splitORParts(parts)
	if err != nil {
		return nil, err
	}
	expandedParts, err := expandWildcardVersion(orParts)
	if err != nil {
		return nil, err
	}
	var orFn Range
	for _, p := range expandedParts {
		var andFn Range
		for _, ap := range p {
			opStr, vStr, err := splitComparatorVersion(ap)
			if err != nil {
				return nil, err
			}
			vr, err := buildVersionRange(opStr, vStr)
			if err != nil {
				return nil, fmt.Errorf("Could not parse Range %q: %s", ap, err)
			}
			rf := vr.rangeFunc()

			// Set function
			if andFn == nil {
				andFn = rf
			} else { // Combine with existing function
				andFn = andFn.AND(rf)
			}
		}
		if orFn == nil {
			orFn = andFn
		} else {
			orFn = orFn.OR(andFn)
		}

	}
	return orFn, nil
}

// splitORParts splits the already cleaned parts by '||'.
// Checks for invalid positions of the operator and returns an
// error if found.
func splitORParts(parts []string) ([][]string, error) {
	var ORparts [][]string
	last := 0
	for i, p := range parts {
		if p == "||" {
			if i == 0 {
				return nil, fmt.Errorf("First element in range is '||'")
			}
			ORparts = append(ORparts, parts[last:i])
			last = i + 1
		}
	}
	if last == len(parts) {
		return nil, fmt.Errorf("Last element in range is '||'")
	}
	ORparts = append(ORparts, parts[last:])
	return ORparts, nil
}

// buildVersionRange takes a slice of 2: operator and version
// and builds a versionRange, otherwise an error.
func buildVersionRange(opStr, vStr string) (*versionRange, error) {
	c := parseComparator(opStr)
	if c == nil {
		return nil, fmt.Errorf("Could not parse comparator %q in %q", opStr, strings.Join([]string{opStr, vStr}, ""))
	}
	v, err := Parse(vStr)
	if err != nil {
		return nil, fmt.Errorf("Could not parse version %q in %q: %s", vStr, strings.Join([]string{opStr, vStr}, ""), err)
	}

	return &versionRange{
		v: v,
		c: c,
	}, nil

}

// inArray checks if a byte is contained in an array of bytes
func inArray(s byte, list []byte) bool {
	for _, el := range list {
		if el == s {
			return true
		}
	}
	return false
}

var excludeFromSplit = []byte{'>', '<', '='}

// splitAndTrim splits a range string by spaces and cleans whitespaces
func splitAndTrim(s string) (result []string) {
	last := 0
	var lastChar byte

	for i := 0; i < len(s); i++ {
		if s[i] == ' ' && !inArray(lastChar, excludeFromSplit) {
			if last < i-1 {
				result = append(result, s[last:i])
			}
			last = i + 1
		} else if s[i] != ' ' {
			lastChar = s[i]
		}
	}
	if last < len(s)-1 {
		result = append(result, s[last:])
	}

	for i, v := range result {
		result[i] = strings.Replace(v, " ", "", -1)
	}

	// parts := strings.Split(s, " ")
	// for _, x := range parts {
	// 	if s := strings.TrimSpace(x); len(s) != 0 {
	// 		result = append(result, s)
	// 	}
	// }
	return
}

// Does not support non-latin1 numbers
func isDigitOrWildcardDigit(r rune) bool {
	switch r {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '*':
		{
			return true
		}
	default:
		{
			return false
		}
	}
}

// splitComparatorVersion splits the comparator from the version.
// Input must be free of leading or trailing spaces.
func splitComparatorVersion(s string) (string, string, error) {
	// Special case because this matches literally any version but does not follow normal patterns.
	if s == "*" {
		return ">=", "0.0.0", nil
	}

	var i int
	i = strings.IndexRune(s, '-')
	if i != -1 {
		return "-", strings.TrimSpace(s[0:i]), nil
	}

	i = strings.IndexFunc(s, isDigitOrWildcardDigit)
	if i == -1 {
		return "", "", fmt.Errorf("could not get version from string: %q", s)
	}
	return strings.TrimSpace(s[0:i]), strings.TrimSpace(s[i:]), nil
}

// getWildcardType will return the type of wildcard that the
// passed version contains
func getWildcardType(vStr string) wildcardType {
	parts := strings.Split(vStr, ".")
	nparts := len(parts)
	wildcard := parts[nparts-1]

	possibleWildcardType := wildcardTypefromInt(nparts)
	if wildcard == "x" || wildcard == "*" {
		return possibleWildcardType
	}

	return noneWildcard
}

func normalizeVersionPart(part string) string {
	var b strings.Builder

	// First, we count.
	var count int

	for _, char := range part {
		switch char {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'x', '*':
			{
				count++
			}
		}
	}

	if count == 0 {
		return "0"
	}

	b.Grow(count)

	for _, char := range part {
		switch char {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			{
				b.WriteRune(char)
			}
		case '*', 'x':
			{
				b.WriteRune('0')
			}
		}
	}

	return b.String()
}

var partI int
var partStartI int = -1
var lastCharI int

var _wildcard wildcardType

var defaultParts = [3]string{"0", "0", "0"}
var secondaryParts = [3]string{"0", "0", "0"}

// createVersionFromWildcard will convert a wildcard version
// into a regular version, replacing 'x's with '0's, handling
// special cases like '1.x.x' and '1.x'
func createVersionFromWildcard(vStr string, parts *[3]string) wildcardType {

	partStartI = -1
	lastCharI = 0
	partI = 0
	_wildcard = noneWildcard
	for i, char := range vStr {

		switch char {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			{
				if partStartI == -1 {
					partStartI = i
				}
				lastCharI = i
				continue
			}
		case '.':
			{
				if partStartI > -1 {
					parts[partI] = normalizeVersionPart(vStr[partStartI:i])
					partStartI = -1
					partI++
				}
			}
		case 'x', '*':
			{
				if partStartI == -1 {
					partStartI = i
				}
				lastCharI = i
				// We want min wildcard
				if _wildcard == noneWildcard {
					switch partI {
					case 0:
						{
							_wildcard = majorWildcard
						}
					case 1:
						{
							_wildcard = minorWildcard
						}
					case 2:
						{
							_wildcard = patchWildcard
						}
					}
				}
			}
		}
	}

	if partI > 2 {
		partI = 2
	}

	parts[partI] = normalizeVersionPart(vStr[partStartI : lastCharI+1])

	return _wildcard
}

func joinTriple(elems [3]string, sep string) string {
	if elems[0] == "" {
		return "0.0.0"
	}

	var b strings.Builder

	if elems[1] == "" {
		return elems[0]
	} else if elems[2] == "" {
		b.Grow(len(elems[0]) + len(elems[1]) + len(sep))
		b.WriteString(elems[0])
		b.WriteString(sep)
		b.WriteString(elems[1])
		return b.String()
	} else {
		b.Grow(len(elems[0]) + len(elems[1]) + len(elems[2]) + len(sep)*2)
		b.WriteString(elems[0])
		b.WriteString(sep)
		b.WriteString(elems[1])
		b.WriteString(sep)
		b.WriteString(elems[2])
		return b.String()
	}

}

var cachedParts = [3]string{"", "", ""}

// incrementMajorVersion will increment the major version
// of the passed version
func incrementMajorVersion(parts [3]string) (string, error) {
	i, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", err
	}
	parts[0] = strconv.Itoa(i + 1)

	return joinTriple(parts, "."), nil
}

// incrementMajorVersion will increment the minor version
// of the passed version
func incrementMinorVersion(parts [3]string) (string, error) {
	i, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", err
	}
	parts[1] = strconv.Itoa(i + 1)
	// parts[2] = "0"

	return joinTriple(parts, "."), nil
}

var zeroTriple = [3]string{"0", "0", "0"}
var resultOperator string = ""
var shouldIncrementVersion bool = false

// expandWildcardVersion will expand wildcards inside versions
// following these rules:
//
// * when dealing with patch wildcards:
// >= 1.2.x    will become    >= 1.2.0
// <= 1.2.x    will become    <  1.3.0
// >  1.2.x    will become    >= 1.3.0
// <  1.2.x    will become    <  1.2.0
// != 1.2.x    will become    <  1.2.0 >= 1.3.0
//
// * when dealing with minor wildcards:
// ~> 1.x      will become    >= 1.0.0
// ^  1.x      will become    >= 1.0.0 <= 2.0.0
// >= 1.x      will become    >= 1.0.0
// <= 1.x      will become    <  2.0.0
// >  1.x      will become    >= 2.0.0
// <  1.0      will become    <  1.0.0
// != 1.x      will become    <  1.0.0 >= 2.0.0
//
// * when dealing with wildcards without
// version operator:
// 1.2.x       will become    >= 1.2.0 < 1.3.0
// 1.x         will become    >= 1.0.0 < 2.0.0
// 1.*         will become    >= 1.0.0 < 2.0.0
func expandWildcardVersion(parts [][]string) ([][]string, error) {
	var expandedParts [][]string
	for _, p := range parts {
		var newParts []string
		for _, ap := range p {
			if strings.ContainsAny(ap, "x^~*-") {
				opStr, vStr, err := splitComparatorVersion(ap)
				if err != nil {
					return nil, err
				}

				versionWildcardType := createVersionFromWildcard(vStr, &defaultParts)
				resultOperator = ""
				shouldIncrementVersion = false

				switch opStr {
				case "-":
					{
						resultOperator = ">="
						createVersionFromWildcard(strings.TrimSpace(ap[strings.IndexRune(ap, '-')+1:]), &secondaryParts)
						newParts = append(newParts, "<"+joinTriple(secondaryParts, "."))
					}
				case "^":
					{
						resultOperator = ">="
						major, _ := strconv.Atoi(defaultParts[0])
						newParts = append(newParts, "<"+strconv.Itoa(major+1)+".0.0")
					}
				case "~":
					{
						switch versionWildcardType {

						// This input doesn't make sense. But, its the internet.
						// People do things that don't make sense.
						// ~*
						case majorWildcard:
							{
								resultOperator = ">="
								defaultParts[0] = "0"
								defaultParts[1] = "0"
								defaultParts[2] = "0"
							}

						case noneWildcard, patchWildcard:
							{
								resultOperator = ">="
								cachedParts[0] = defaultParts[0]
								cachedParts[2] = "0"

								patch, _ := strconv.Atoi(defaultParts[1])

								cachedParts[1] = strconv.Itoa(patch + 1)

								newParts = append(newParts, "<"+joinTriple(cachedParts, "."))
							}

						case minorWildcard:
							{
								resultOperator = ">="
								cachedParts[1] = "0"
								cachedParts[2] = "0"

								patch, _ := strconv.Atoi(defaultParts[0])

								cachedParts[0] = strconv.Itoa(patch + 1)

								newParts = append(newParts, "<"+joinTriple(cachedParts, "."))
							}

						}

					}
				case ">":
					resultOperator = ">="
					shouldIncrementVersion = true
				case "~>", ">=":
					resultOperator = ">="
				case "<":
					resultOperator = "<"
				case "<=":
					resultOperator = "<"
					shouldIncrementVersion = true
				case "", "=", "==":
					newParts = append(newParts, ">="+joinTriple(defaultParts, "."))
					resultOperator = "<"
					shouldIncrementVersion = true
				case "!=", "!":
					newParts = append(newParts, "<"+joinTriple(defaultParts, "."))
					resultOperator = ">="
					shouldIncrementVersion = true
				}

				var resultVersion string
				if shouldIncrementVersion {
					switch versionWildcardType {
					case patchWildcard:
						resultVersion, _ = incrementMinorVersion(defaultParts)
					case minorWildcard:
						resultVersion, _ = incrementMajorVersion(defaultParts)
					}
				} else {
					resultVersion = joinTriple(defaultParts, ".")
				}

				newParts = append(newParts, resultOperator+resultVersion)
				// Handle "0", "1", "2", "3", "4", "5", "6", "7", "8", "9"
			} else if isNumbersOrSpacesOnly(ap) {
				createVersionFromWildcard(ap, &defaultParts)
				newParts = append(newParts, joinTriple(defaultParts, "."))
			} else {
				newParts = append(newParts, ap)
			}

		}
		expandedParts = append(expandedParts, newParts)
	}

	return expandedParts, nil
}

func isNumbersOrSpacesOnly(ap string) bool {
	for _, r := range ap {
		if !(r == ' ' || (r >= '0' && r <= '9')) {
			return false
		}
	}

	return true
}

func parseComparator(s string) comparator {
	switch s {
	case "==":
		fallthrough
	case "":
		fallthrough
	case "=":
		return compEQ
	case ">":
		return compGT
	case ">=":
		return compGE
	case "<":
		return compLT
	case "<=":
		return compLE
	case "!":
		fallthrough
	case "!=":
		return compNE
	}

	return nil
}

// MustParseRange is like ParseRange but panics if the range cannot be parsed.
func MustParseRange(s string) Range {
	r, err := ParseRange(s)
	if err != nil {
		panic(`semver: ParseRange(` + s + `): ` + err.Error())
	}
	return r
}
