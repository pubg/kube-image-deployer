package util

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var ErrNotFound = errors.New("not found")

// GetHighestVersionWithFilter versions는 version 목록이다.
// filter는 *(asterisk)를 숫자(\d+)로 대입하는 regexp로 변환된다.
// 예를 들어, filter가 "1.2.*"이면, 1.2.3이나 1.2.4라는 버전을 찾는다.
// 그리고 그 중 *의 위치에 해당하는 숫자가 가장 큰 버전을 반환한다.
// *은 여럿일 수 있으며 왼쪽에서 오른쪽으로 동일 위치에서 더 큰 버전을 찾는다.
// 예를 들어, filter가 "1.*.*"이면, 1.1.5와 1.2.0 중 1.2.0을 반환한다.
func GetHighestVersionWithFilter(versions []string, filter string) (string, error) {
	highestTag := ""
	highestNumbers := []int64{}
	regexString := fmt.Sprintf("^%s$", strings.ReplaceAll(regexp.QuoteMeta(filter), `\*`, `(\d+)`))
	patt, err := regexp.Compile(regexString)

	if nil != err {
		return "", err
	}

	for _, tag := range versions {
		matches := patt.FindStringSubmatch(tag)

		if tag == "" || len(matches) < 2 {
			continue
		}

		numbers := []int64{}
		valid := true

		for idx, match := range matches {
			if idx == 0 { // 첫번째 매치는 무시한다.
				continue
			}
			if number, err := strconv.ParseInt(match, 10, 64); err != nil {
				valid = false // int64 범위를 벗어나는 비정상 태그는 건너뛴다
				break
			} else {
				numbers = append(numbers, number)
			}
		}

		if !valid {
			continue
		}

		// 왼쪽 자릿수부터 사전식 비교: 크면 교체, 작으면 유지, 같으면 다음 자릿수로
		if highestTag == "" || compareNumbers(numbers, highestNumbers) > 0 {
			highestTag, highestNumbers = tag, numbers
		}
	}

	if highestTag == "" {
		return "", ErrNotFound
	}

	return highestTag, nil
}

// compareNumbers 두 버전 자릿수 배열을 사전식으로 비교한다.
// a > b이면 1, a < b이면 -1, 같으면 0을 반환한다.
func compareNumbers(a, b []int64) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] > b[i] {
			return 1
		}
		if a[i] < b[i] {
			return -1
		}
	}
	// 앞자리가 모두 같으면 자릿수가 많은 쪽을 더 높은 버전으로 본다
	if len(a) > len(b) {
		return 1
	}
	if len(a) < len(b) {
		return -1
	}
	return 0
}
