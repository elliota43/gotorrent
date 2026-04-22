package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/elliota43/gotorrent/bencode"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s <file.torrent>", os.Args[0])
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	dec := bencode.NewDecoder(f)
	v, err := dec.Decode()
	if err != nil {
		log.Fatal(err)
	}

	printValue(v, 0)
}

func printValue(v any, indent int) {
	pad := strings.Repeat(" ", indent)

	switch x := v.(type) {
	case int64:
		fmt.Printf("%sint: %d\n", pad, x)

	case []byte:
		fmt.Printf("%sbytes: %q\n", pad, string(x))

	case bencode.List:
		fmt.Printf("%slist[%d]:\n", pad, len(x))
		for i, item := range x {
			fmt.Printf("%s  [%d]\n", pad, i)
			printValue(item, indent+2)
		}

	case bencode.Dict:
		fmt.Printf("%sdict[%d]:\n", pad, len(x))

		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			fmt.Printf("%s  %q:\n", pad, k)
			printValue(x[k], indent+2)
		}

	default:
		fmt.Printf("%sunknown %T: %#v\n", pad, v, v)
	}
}
