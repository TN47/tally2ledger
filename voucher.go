package main

import "fmt"
import "time"
import "regexp"
import "strconv"
import "strings"

import "github.com/prataprc/goparsec"
import "github.com/prataprc/golog"

type Voucher interface {
	Type() string

	Rewrite(rules map[string]interface{})

	ToLedger() []string
}

func newvoucher(fields ...parsec.ParsecNode) Voucher {
	switch fields[4].(string) {
	case "Jrnl":
		return NewJournal(fields...)
	case "Rcpt":
		return NewReceipt(fields...)
	case "Pymt":
		return NewPayment(fields...)
	case "Ctra":
		return NewContra(fields...)
	default:
		panic(fmt.Errorf("unsupported voucher type : %s", fields[4]))
	}
	return nil
}

func formatfields(fields ...parsec.ParsecNode) string {
	items := []string{}
	for i, field := range fields {
		items = append(items, fmt.Sprintf("%v:%v", i, field))
	}
	return strings.Join(items, ", ")
}

type Receipt struct {
	Date     time.Time
	Payee    string
	Debtor   string
	Creditor string
	Debit    float64
	Credit   float64
	Notes    []string
	parsed   bool
}

func NewReceipt(fields ...parsec.ParsecNode) (r *Receipt) {
	notesorignore := []int{2, 3, 5, 7, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18,
		19, 20, 21}
	if fields[4].(string) != "Rcpt" {
		panic("impossible situation")
	}

	r = &Receipt{
		Date:     parsetime(fields[0]),
		Debtor:   fields[8].(string),
		Creditor: fields[1].(string),
		Debit:    parsefloat(fields[6]),
		Credit:   -parsefloat(fields[6]),
		Notes:    getnotes(fields, notesorignore),
		parsed:   true,
	}
	return r
}

func (r *Receipt) Type() string {
	return "Receipt"
}

func (r *Receipt) Rewrite(rules map[string]interface{}) {
	for from, to := range rules["accountname"].(map[string]interface{}) {
		if r.Debtor == from {
			r.Debtor = to.(string)
		} else if r.Creditor == from {
			r.Creditor = to.(string)
		}
	}
	for from, to := range rules["payee"].(map[string]interface{}) {
		if r.Payee == from {
			r.Payee = to.(string)
		}
	}
}

func (r *Receipt) ToLedger() []string {
	tm := r.Date.Format("2006-01-02")
	lines := []string{
		fmt.Sprintf("%v  %v", tm, "Payee"),
		fmt.Sprintf("    %-40v  %.2f", r.Debtor, r.Debit),
		fmt.Sprintf("    %-40v  %.2f", r.Creditor, r.Credit),
		fmt.Sprintf("    ; Receipt"),
	}
	for _, note := range r.Notes {
		lines = append(lines, fmt.Sprintf("    ; %v", note))
	}
	return lines
}

var njournals int

type Journal struct {
	Date      time.Time
	Payee     string
	Debtors   []string
	Creditors []string
	Debits    []float64
	Credits   []float64
	Notes     []string
	parsed    bool
}

func NewJournal(fields ...parsec.ParsecNode) (j *Journal) {
	njournals++

	if fields[4].(string) != "Jrnl" {
		panic("impossible situation")
	}

	// gather fields
	date := parsetime(fields[0])
	debtors := []string{strings.Trim(fields[1].(string), " ")}
	debits := []float64{-parsefloat(fields[5])}
	// credit offset
	co, err := jrnlcreditoffset(fields)
	if err != nil {
		log.Errorf("%v\n", err)
		return nil
	}
	creditors := []string{}
	credits := []float64{}
	for ; co+8 < len(fields); co += 8 {
		creditor := strings.Trim(fields[co+1].(string), " ")
		if creditor == "" {
			fmsg := "%v creditor field %v is empty\n"
			log.Errorf(fmsg, njournals, co+1)
			return nil
		}
		creditors = append(creditors, creditor)
		credits = append(credits, -parsefloat(fields[co+7]))
		checknodes := []parsec.ParsecNode{fields[co+0]}
		warncontent(append(checknodes, fields[co+2:co+7]...))
	}
	// gather notes
	notes := getfnotes(fields[co:])

	j = &Journal{
		Date: date, Debtors: debtors, Creditors: creditors,
		Debits: debits, Credits: credits, parsed: true,
		Notes: notes,
	}
	return j
}

func (j *Journal) Type() string {
	return "Journal"
}

func (j *Journal) Rewrite(rules map[string]interface{}) {
	for from, to := range rules["accountname"].(map[string]interface{}) {
		for i, debtor := range j.Debtors {
			if debtor == from {
				j.Debtors[i] = to.(string)
			}
		}
		for i, creditor := range j.Creditors {
			if creditor == from {
				j.Creditors[i] = to.(string)
			}
		}
	}
	for from, to := range rules["payee"].(map[string]interface{}) {
		if j.Payee == from {
			j.Payee = to.(string)
		}
	}
}

func (j *Journal) ToLedger() []string {
	tm := j.Date.Format("2006-01-02")
	lines := []string{
		fmt.Sprintf("%v  %v ; Journal", tm, "Payee"),
	}
	for i, debtor := range j.Debtors {
		line := fmt.Sprintf("    %-40v  %.2f", debtor, j.Debits[i])
		lines = append(lines, line)
	}
	for i, creditor := range j.Creditors {
		line := fmt.Sprintf("    %-40v  %.2f", creditor, j.Credits[i])
		lines = append(lines, line)
	}
	for _, note := range j.Notes {
		lines = append(lines, fmt.Sprintf("    ; %v", note))
	}
	return lines
}

type Payment struct {
	Date     time.Time
	Payee    string
	Debtor   string
	Creditor string
	Debit    float64
	Credit   float64
	Notes    []string
	parsed   bool
}

func NewPayment(fields ...parsec.ParsecNode) (p *Payment) {
	notesorignore := []int{2, 3, 6, 7, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18,
		19, 20, 21}
	if fields[4].(string) != "Pymt" {
		panic("impossible situation")
	}

	p = &Payment{
		Date:     parsetime(fields[0]),
		Debtor:   fields[1].(string),
		Creditor: fields[8].(string),
		Debit:    -parsefloat(fields[5]),
		Credit:   parsefloat(fields[5]),
		Notes:    getnotes(fields, notesorignore),
		parsed:   true,
	}
	return p
}

func (p *Payment) Type() string {
	return "Payment"
}

func (p *Payment) Rewrite(rules map[string]interface{}) {
	for from, to := range rules["accountname"].(map[string]interface{}) {
		if p.Debtor == from {
			p.Debtor = to.(string)
		} else if p.Creditor == from {
			p.Creditor = to.(string)
		}
	}
	for from, to := range rules["payee"].(map[string]interface{}) {
		if p.Payee == from {
			p.Payee = to.(string)
		}
	}
}

func (p *Payment) ToLedger() []string {
	tm := p.Date.Format("2006-01-02")
	lines := []string{
		fmt.Sprintf("%v  %v", tm, "Payee"),
		fmt.Sprintf("    %-40v  %.2f", p.Debtor, p.Debit),
		fmt.Sprintf("    %-40v  %.2f", p.Creditor, p.Credit),
		fmt.Sprintf("    ; Payment"),
	}
	for _, note := range p.Notes {
		lines = append(lines, fmt.Sprintf("    ; %v", note))
	}
	return lines
}

type Contra struct {
	Date     time.Time
	Payee    string
	Debtor   string
	Creditor string
	Debit    float64
	Credit   float64
	Notes    []string
	parsed   bool
}

func NewContra(fields ...parsec.ParsecNode) (c *Contra) {
	notesorignore := []int{2, 3, 5, 7, 8, 9, 10, 11, 13, 14, 15, 16, 17, 18,
		19, 20, 21}
	if fields[4].(string) != "Ctra" {
		panic("impossible situation")
	}

	c = &Contra{
		Date:     parsetime(fields[0]),
		Debtor:   fields[1].(string),
		Creditor: fields[12].(string),
		Debit:    parsefloat(fields[6]),
		Credit:   -parsefloat(fields[6]),
		Notes:    getnotes(fields, notesorignore),
		parsed:   true,
	}
	return c
}

func (c *Contra) Type() string {
	return "Contra"
}

func (c *Contra) Rewrite(rules map[string]interface{}) {
	for from, to := range rules["accountname"].(map[string]interface{}) {
		if c.Debtor == from {
			c.Debtor = to.(string)
		} else if c.Creditor == from {
			c.Creditor = to.(string)
		}
	}
	for from, to := range rules["payee"].(map[string]interface{}) {
		if c.Payee == from {
			c.Payee = to.(string)
		}
	}
}

func (c *Contra) ToLedger() []string {
	tm := c.Date.Format("2006-01-02")
	lines := []string{
		fmt.Sprintf("%v  %v", tm, "Payee"),
		fmt.Sprintf("    %-40v  %.2f", c.Debtor, c.Debit),
		fmt.Sprintf("    %-40v  %.2f", c.Creditor, c.Credit),
		fmt.Sprintf("    ; Contra"),
	}
	for _, note := range c.Notes {
		lines = append(lines, fmt.Sprintf("    ; %v", note))
	}
	return lines
}

func getnotes(fields []parsec.ParsecNode, indexes []int) []string {
	notes := []string{}
	for _, index := range indexes {
		if index >= len(fields) {
			continue
		}
		if s, ok := fields[index].(string); ok {
			if s != "" {
				notes = append(notes, s)
			}
		}
	}
	return notes
}

func getfnotes(fields []parsec.ParsecNode) []string {
	notes := []string{}
	for _, field := range fields {
		if s, ok := field.(string); ok {
			if s != "" && strings.HasPrefix(s, "(No. ") == false {
				notes = append(notes, s)
			}
		}
	}
	return notes
}

func parsefloat(field parsec.ParsecNode) float64 {
	t := field.(*parsec.Terminal)
	credit, err := strconv.ParseFloat(string(t.Value), 64)
	if err != nil {
		panic(fmt.Errorf("unable to parse amount: %v", string(t.Value)))
	}
	return credit
}

func parsetime(field parsec.ParsecNode) time.Time {
	t := field.(*parsec.Terminal)
	re, _ := regexp.Compile("([0-9]{1,2})-([0-9]{1,2})-([0-9]{4})")
	matches := re.FindStringSubmatch(string(t.Value))
	date, _ := strconv.Atoi(matches[1])
	month, _ := strconv.Atoi(matches[2])
	year, _ := strconv.Atoi(matches[3])
	return time.Date(year, time.Month(month), date, 0, 0, 0, 0, time.Local)
}

func warncontent(fields []parsec.ParsecNode) {
	for _, field := range fields {
		if s, ok := field.(string); ok && strings.Trim(s, " ") == "" {
			continue
		} else if _, ok = field.(parsec.MaybeNone); ok {
			continue
		}
		log.Warnf("unknown field %v\n", field)
	}
}

func jrnlcreditoffset(fields []parsec.ParsecNode) (int, error) {
	// credit offset based on based on size
	var co int
	switch len(fields) {
	case 18:
		co = 7
	case 22, 30, 38, 46, 54, 62, 70:
		// credit offset based on Cheque/DD
		co = jrnlchequeordd(fields)
	default:
		fmsg := "%v number of journal fields == %v"
		err := fmt.Errorf(fmsg, njournals, len(fields))
		return 0, err
	}
	return co, nil
}

func jrnlchequeordd(fields []parsec.ParsecNode) int {
	if s, ok := fields[7].(string); ok {
		if strings.Contains(strings.ToLower(strings.Trim(s, " ")), "cheque") {
			return 11
		}
	}
	return 7
}
