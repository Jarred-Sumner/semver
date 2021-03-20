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

// splitAndTrim splits a range string by spaces and cleans whitespaces
func splitAndTrim(s string) []string {
	var lastChar rune
	// var lastChar byte

	// First lets count the number of non-consecutive spaces
	count := 1
	var i int
	var r rune

	for i, r = range s {
		if i > 0 && r == ' ' && lastChar != ' ' && lastChar != '<' && lastChar != '>' && lastChar != '=' {
			count++
		}
		lastChar = r
	}

	if i > 0 && lastChar != ' ' && lastChar != '<' && lastChar != '>' && lastChar != '=' {
		count++
	}

	lastChar = 0

	result := make([]string, 0, count)
	head := 0

	for i, r = range s {
		// Next part!
		if i > 0 && r == ' ' && lastChar != ' ' && lastChar != '<' && lastChar != '>' && lastChar != '=' {
			// TODO: use string builder to prevent memory allocations
			result = append(result, strings.ReplaceAll(s[head:i], " ", ""))
			head = i
		}
		lastChar = r
	}

	if i > 0 && lastChar != ' ' && lastChar != '<' && lastChar != '>' && lastChar != '=' {

		// TODO: use string builder to prevent memory allocations
		content := strings.ReplaceAll(s[head:i+1], " ", "")
		if content != "" {
			result = append(result, content)
		}

	}

	return result
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
	if i != -1 && !strings.ContainsAny(s, "^+|><=") {
		return "-", strings.TrimSpace(s[0:i]), nil
	}

	i = strings.IndexFunc(s, isDigitOrWildcardDigit)
	if i == -1 {
		return "", "", fmt.Errorf("could not get version from string: %q", s)
	}
	return strings.TrimSpace(s[0:i]), strings.TrimSpace(s[i:]), nil
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

type versionParts = [4]string

// createVersionFromWildcard will convert a wildcard version
// into a regular version, replacing 'x's with '0's, handling
// special cases like '1.x.x' and '1.x'
func createVersionFromWildcard(vStr string) (versionParts, wildcardType, bool) {
	parts := [4]string{"0", "0", "0", ""}
	partI := 0
	partStartI := -1
	lastCharI := 0
	_wildcard := noneWildcard
	isValid := true
	isDone := false

	for i, char := range vStr {
		if isDone {
			break
		}

		switch char {
		case ' ':
			{

			}
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			{
				if partStartI == -1 {
					partStartI = i
				}
				lastCharI = i
			}

		case '.':
			{
				if partStartI > -1 && partI <= 2 {
					parts[partI] = normalizeVersionPart(vStr[partStartI:i])
					partStartI = -1
					partI++
					// "fo.o.b.ar"
				} else if partI > 2 || partStartI == -1 {
					isValid = false
					isDone = true
					break
				}
			}
		case '-', '+':
			{
				if partI == 2 && partStartI > -1 {
					parts[partI] = normalizeVersionPart(vStr[partStartI:i])
					_wildcard = noneWildcard
					partStartI = i
					partI = 3
					isDone = true
					break
				} else {
					isValid = false
					isDone = true
					break
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

		default:
			{
				lastCharI = 0
				isValid = false
				isDone = true
				break
			}
		}
	}

	if isValid {
		isValid = partI > -1
	}

	if partStartI == -1 {
		partStartI = 0
	}

	if lastCharI == -1 || partStartI > lastCharI {
		lastCharI = len(vStr) - 1
	}

	// Where did we leave off?
	switch partI {

	// That means they used a match like this:
	// "1"
	// So its a wildcard minor
	case 0:
		{
			if _wildcard == noneWildcard {
				_wildcard = minorWildcard
			}

			parts[0] = normalizeVersionPart(vStr[partStartI : lastCharI+1])

		}

	case 3:
		{
			parts[3] = vStr[partStartI:]
		}

	default:
		{

			parts[partI] = normalizeVersionPart(vStr[partStartI : lastCharI+1])
		}
	}

	return parts, _wildcard, isValid
}

func joinParts(elems versionParts, sep string) string {
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
	} else if elems[3] == "" {
		b.Grow(len(elems[0]) + len(elems[1]) + len(elems[2]) + len(sep)*2)
		b.WriteString(elems[0])
		b.WriteString(sep)
		b.WriteString(elems[1])
		b.WriteString(sep)
		b.WriteString(elems[2])
		return b.String()
	} else {
		b.Grow(len(elems[0]) + len(elems[1]) + len(elems[2]) + len(elems[3]) + len(sep)*2)
		b.WriteString(elems[0])
		b.WriteString(sep)
		b.WriteString(elems[1])
		b.WriteString(sep)
		b.WriteString(elems[2])
		b.WriteString(elems[3])
		return b.String()
	}

}

// incrementMajorVersion will increment the major version
// of the passed version
func incrementMajorVersion(parts versionParts) (string, error) {
	i, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", err
	}
	parts[0] = strconv.Itoa(i + 1)

	return joinParts(parts, "."), nil
}

// incrementMajorVersion will increment the minor version
// of the passed version
func incrementMinorVersion(parts versionParts) (string, error) {
	i, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", err
	}
	parts[1] = strconv.Itoa(i + 1)
	// parts[2] = "0"

	return joinParts(parts, "."), nil
}

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

				var cachedParts = versionParts{"", "", "", ""}
				defaultParts, versionWildcardType, _ := createVersionFromWildcard(vStr)
				var resultOperator string = ""
				var shouldIncrementVersion bool = false

				switch opStr {
				case "-":
					{
						resultOperator = ">="
						secondaryParts, _, _ := createVersionFromWildcard(strings.TrimSpace(ap[strings.IndexRune(ap, '-')+1:]))
						newParts = append(newParts, "<"+joinParts(secondaryParts, "."))
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

								newParts = append(newParts, "<"+joinParts(cachedParts, "."))
							}

						case minorWildcard:
							{
								resultOperator = ">="
								cachedParts[1] = "0"
								cachedParts[2] = "0"

								patch, _ := strconv.Atoi(defaultParts[0])

								cachedParts[0] = strconv.Itoa(patch + 1)

								newParts = append(newParts, "<"+joinParts(cachedParts, "."))
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
					newParts = append(newParts, ">="+joinParts(defaultParts, "."))
					resultOperator = "<"
					shouldIncrementVersion = true
				case "!=", "!":
					newParts = append(newParts, "<"+joinParts(defaultParts, "."))
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
					resultVersion = joinParts(defaultParts, ".")
				}

				newParts = append(newParts, resultOperator+resultVersion)
				// Handle "0", "1", "2", "3", "4", "5", "6", "7", "8", "9"
			} else if isNumbersOrSpacesOnly(ap) {
				defaultParts, _, _ := createVersionFromWildcard(ap)
				newParts = append(newParts, joinParts(defaultParts, "."))
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
