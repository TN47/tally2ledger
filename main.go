package main

import "os"
import "fmt"
import "flag"
import "sort"
import "bytes"
import "reflect"
import "strings"
import "io/ioutil"
import "encoding/json"

import "github.com/prataprc/golog"
import "github.com/prataprc/goparsec"

var options struct {
	textfile string
	ldgfile  string
	rewrite  string
	loglevel string
}

func argparse() []string {
	f := flag.NewFlagSet("tally2ledger", flag.ExitOnError)
	f.Usage = func() {
		fmsg := "Usage of command: %v [ARGS]\n"
		fmt.Printf(fmsg, os.Args[0])
		f.PrintDefaults()
	}

	f.StringVar(&options.textfile, "text", "",
		"tally exported textfile file to parse and convert to ledger.")
	f.StringVar(&options.ldgfile, "o", "",
		"targer ledger filename")
	f.StringVar(&options.rewrite, "rewrite", "",
		"rewrite rules from tally to ledger")
	f.StringVar(&options.loglevel, "log", "info",
		"log level.")
	f.Parse(os.Args[1:])

	if options.textfile == "" {
		log.Errorf("Please specify the tally exported text file\n")
		os.Exit(1)
	}
	if options.ldgfile == "" {
		log.Errorf("Please specify the tally exported text file\n")
		os.Exit(1)
	}

	args := f.Args()
	return args
}

func main() {
	argparse()

	logsetts := map[string]interface{}{
		"log.level":      options.loglevel,
		"log.file":       "",
		"log.timeformat": "",
		"log.prefix":     "%v:",
	}
	log.SetLogger(nil, logsetts)

	data, err := ioutil.ReadFile(options.textfile)
	if err != nil {
		log.Errorf("%v\n", err)
		os.Exit(1)
	}
	data = data[3:] // skip BOM
	data = bytes.Replace(data, []byte{0xd, 0xa}, []byte{}, -1)

	// Parse tally vouchers
	vouchers, err := tallyvouchers(data)
	if err != nil {
		os.Exit(1)
	}
	logstats(vouchers)

	// Apply rewrite rules.
	if options.rewrite != "" {
		log.Infof("Applying rewrite rules from %q\n", options.rewrite)
		if err := applyrules(vouchers); err != nil {
			os.Exit(2)
		}
	}

	// Transform tally vouchers to  ledger
	if err := toledger(vouchers); err != nil {
		os.Exit(3)
	}
}

func tallyvouchers(data []byte) ([]Voucher, error) {
	// parser combinators
	ydate := parsec.Token("[0-9]{1,2}-[0-9]{1,2}-[0-9]{4}", "DATE")
	yhackstr := parsec.And(
		func(nodes []parsec.ParsecNode) parsec.ParsecNode {
			s := nodes[0].(string)
			s = s[1 : len(s)-1]
			return s
		},
		parsec.String(),
	)
	yterm := parsec.OrdChoice(
		vector2scalar,
		yhackstr, parsec.Float(), ydate)
	y := parsec.Kleene(
		nil,
		parsec.Maybe(maybenode, yterm), parsec.Token(`,`, "FIELDSEP"),
	)

	scanner := parsec.NewScanner(data)
	nodes, scanner := y(scanner)
	if scanner.Endof() == false {
		err := fmt.Errorf("expected eof %v\n", scanner.GetCursor())
		log.Errorf("%v\n", err)
		return nil, err
	}
	vouchers := []Voucher{}
	fields := []parsec.ParsecNode{}
	for _, term := range nodes.([]parsec.ParsecNode) {
		if val, ok := term.(string); ok {
			if strings.HasPrefix(val, "(No. :") {
				fields = append(fields, term)
				voucher := newvoucher(fields...)
				if voucher == nil || reflect.ValueOf(voucher).IsNil() {
					return nil, fmt.Errorf("invalid voucher")
				}
				vouchers = append(vouchers, voucher)
				fields = fields[:0]
				continue
			}
		}
		fields = append(fields, term)
	}
	return vouchers, nil
}

func applyrules(vouchers []Voucher) error {
	var rules map[string]interface{}

	data, err := ioutil.ReadFile(options.rewrite)
	if err != nil {
		log.Errorf("%v\n", err)
		return err
	}
	if err := json.Unmarshal(data, &rules); err != nil {
		log.Errorf("Unmarshal(%q): %v", options.rewrite)
		return err
	}
	for _, voucher := range vouchers {
		voucher.Rewrite(rules)
	}
	return nil
}

func toledger(vouchers []Voucher) error {
	os.Remove(options.ldgfile)
	flags := os.O_APPEND | os.O_CREATE | os.O_WRONLY
	fd, err := os.OpenFile(options.ldgfile, flags, 0660)
	if err != nil {
		log.Errorf("%v\n", err)
		return err
	}
	newline := fmt.Sprintln()
	for _, voucher := range vouchers {
		outdata := []byte(strings.Join(voucher.ToLedger(), newline))
		outdata = append(outdata, newline...)
		outdata = append(outdata, newline...)
		_, err := fd.Write(outdata)
		if err != nil {
			log.Errorf("%v\n", err)
			return err
		}
	}
	return nil
}

func logstats(vouchers []Voucher) {
	stats := map[string]int{}
	for _, voucher := range vouchers {
		typ := voucher.Type()
		if _, ok := stats[typ]; ok == false {
			stats[typ] = 0
		}
		stats[typ] += 1
	}
	types := []string{}
	for typ := range stats {
		types = append(types, typ)
	}
	sort.Strings(types)
	total := 0
	for _, typ := range types {
		total += stats[typ]
		log.Infof("%v vouchers : %v\n", typ, stats[typ])
	}
	log.Infof("Total vouchers : %v\n", total)
}

func maybenode(nodes []parsec.ParsecNode) parsec.ParsecNode {
	if nodes == nil || len(nodes) == 0 {
		return nil
	}
	return nodes[0]
}

func vector2scalar(nodes []parsec.ParsecNode) parsec.ParsecNode {
	return nodes[0]
}
