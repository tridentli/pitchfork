package pitchfork

import (
	"fmt"
	"time"
)

func ExampletmpFmtDateMin_daychange() {
	time_layout := "2006-01-02"

	a, _ := time.Parse(time_layout, "2017-01-05")
	b, _ := time.Parse(time_layout, "2017-01-07")

	out := tmpFmtDateMin(a, b, "")

	fmt.Println(out)
	// Output: 2017-01-05...07
}

func ExampletmpFmtDateMin_monthchange() {
	time_layout := "2006-01-02"

	a, _ := time.Parse(time_layout, "2017-01-31")
	b, _ := time.Parse(time_layout, "2017-02-03")

	out := tmpFmtDateMin(a, b, "")

	fmt.Println(out)
	// Output: 2017-01-31 - 02-03
}

func ExampletmpFmtDateMin_yearchange() {
	time_layout := "2006-01-02"

	a, _ := time.Parse(time_layout, "2017-01-31")
	b, _ := time.Parse(time_layout, "2018-02-01")

	out := tmpFmtDateMin(a, b, "")

	fmt.Println(out)
	// Output: 2017-01-31 - 2018-02-01
}

func ExampletmpFmtDateMin_sameyear() {
	time_layout := "2006-01-02"

	a, _ := time.Parse(time_layout, "2017-02-01")
	b, _ := time.Parse(time_layout, "2017-02-05")

	out := tmpFmtDateMin(a, b, "2017")

	fmt.Println(out)
	// Output: 02-01...05
}
