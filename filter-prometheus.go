//
// Copyright (c) 2020 Gilles Chehade <gilles@poolp.org>
//
// Permission to use, copy, modify, and distribute this software for any
// purpose with or without fee is hereby granted, provided that the above
// copyright notice and this permission notice appear in all copies.
//
// THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
// WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
// ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
// WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
// ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
// OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
//

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"log"
	"net/http"
)

var exporter *string
var registerSMTPIn bool = false
var registerSMTPOut bool = false

type session struct {
	id string

	inet4 bool
	inet6 bool
	unix  bool

	auth bool
	tls  bool
}

var sessions = make(map[string]*session)

type metrics struct {
	sessionsActive uint64
	sessionsTotal  uint64

	sessionsUnixActive  uint64
	sessionsInet4Active uint64
	sessionsInet6Active uint64

	sessionsUnixTotal  uint64
	sessionsInet4Total uint64
	sessionsInet6Total uint64

	sessionsTLSActive uint64
	sessionsTLSTotal  uint64

	sessionsAuthActive   uint64
	sessionsAuthTotal    uint64
	sessionsAuthFailures uint64

	txActive        uint64
	txCommitTotal   uint64
	txRollbackTotal uint64
	txTotal         uint64
}

var smtpIn = metrics{}
var smtpOut = metrics{}

var reporters = map[string]func(*session, string, []string){
	"link-connect":    linkConnect,
	"link-disconnect": linkDisconnect,
	"link-tls":        linkTLS,
	"link-auth":       linkAuth,
	"tx-reset":        txReset,
	"tx-begin":        txBegin,
	"tx-mail":         txMail,
	"tx-rcpt":         txRcpt,
	"tx-commit":       txCommit,
	"tx-rollback":     txRollback,
}

func getMetrics(subsystem string) *metrics {
	if subsystem == "smtp-in" {
		return &smtpIn
	} else if subsystem == "smtp-out" {
		return &smtpOut
	}
	log.Fatal("invalid input, shouldn't happen")
	return &metrics{}
}

func linkConnect(s *session, subsystem string, params []string) {
	if len(params) != 4 {
		log.Fatal("invalid input, shouldn't happen")
	}
	m := getMetrics(subsystem)

	m.sessionsActive++
	m.sessionsTotal++

	src := params[2]
	if !strings.HasPrefix(src, "unix:") {
		if src[0] == '[' {
			m.sessionsInet6Active++
			m.sessionsInet6Total++
			s.inet6 = true
		} else {
			m.sessionsInet4Active++
			m.sessionsInet4Total++
			s.inet4 = true
		}
	} else {
		m.sessionsUnixActive++
		m.sessionsUnixTotal++
		s.unix = true
	}
}

func linkDisconnect(s *session, subsystem string, params []string) {
	if len(params) != 0 {
		log.Fatal("invalid input, shouldn't happen")
	}
	m := getMetrics(subsystem)
	if s.inet4 {
		m.sessionsInet4Active--
	} else if s.inet6 {
		m.sessionsInet6Active--
	} else if s.unix {
		m.sessionsUnixActive--
	}

	if s.auth {
		m.sessionsAuthActive--
	}

	if s.tls {
		m.sessionsTLSActive--
	}

	m.sessionsActive--

	delete(sessions, s.id)
}

func linkTLS(s *session, subsystem string, params []string) {
	if len(params) != 1 {
		log.Fatal("invalid input, shouldn't happen")
	}
	m := getMetrics(subsystem)
	m.sessionsTLSActive++
	m.sessionsTLSTotal++
	s.tls = true
}

func linkAuth(s *session, subsystem string, params []string) {
	if len(params) != 2 {
		log.Fatal("invalid input, shouldn't happen")
	}
	m := getMetrics(subsystem)

	if params[1] != "pass" {
		m.sessionsAuthFailures++
		return
	}
	m.sessionsAuthActive++
	m.sessionsAuthTotal++
	s.auth = true
}

func txReset(s *session, subsystem string, params []string) {
	if len(params) != 1 {
		log.Fatal("invalid input, shouldn't happen")
	}
	m := getMetrics(subsystem)
	m.txActive--
}

func txBegin(s *session, subsystem string, params []string) {
	if len(params) != 1 {
		log.Fatal("invalid input, shouldn't happen")
	}
	m := getMetrics(subsystem)
	m.txActive++
	m.txTotal++
}

func txMail(s *session, subsystem string, params []string) {
	if len(params) < 3 {
		log.Fatal("invalid input, shouldn't happen")
	}
	//m := getMetrics(subsystem)
	status := params[1]

	if status != "ok" {
		return
	}
}

func txRcpt(s *session, subsystem string, params []string) {
	if len(params) < 3 {
		log.Fatal("invalid input, shouldn't happen")
	}
	//m := getMetrics(subsystem)
	status := params[1]

	if status != "ok" {
		return
	}
}

func txCommit(s *session, subsystem string, params []string) {
	m := getMetrics(subsystem)
	m.txCommitTotal++
}

func txRollback(s *session, subsystem string, params []string) {
	m := getMetrics(subsystem)
	m.txRollbackTotal++
}

func filterInit() {
	if registerSMTPIn {
		fmt.Printf("register|report|smtp-in|*\n")
	}
	if registerSMTPOut {
		fmt.Printf("register|report|smtp-out|*\n")
	}
	fmt.Println("register|ready")
}

func trigger(actions map[string]func(*session, string, []string), atoms []string) {
	if atoms[4] == "link-connect" {
		// special case to simplify subsequent code
		s := session{}
		s.id = atoms[5]
		sessions[s.id] = &s
	}

	s, ok := sessions[atoms[5]]
	if !ok {
		return
	}

	if v, ok := actions[atoms[4]]; ok {
		v(s, atoms[3], atoms[6:])
	}
}

func skipConfig(scanner *bufio.Scanner) {
	for {
		if !scanner.Scan() {
			os.Exit(0)
		}
		line := scanner.Text()
		if line == "config|subsystem|smtp-in" {
			registerSMTPIn = true
		}
		if line == "config|subsystem|smtp-out" {
			registerSMTPOut = true
		}
		if line == "config|ready" {
			return
		}
	}
}

func smtpMetricsHandlers(direction string, w http.ResponseWriter, r *http.Request) {
	m := getMetrics(direction)

	fmt.Fprintf(w, "# HELP smtpd_sessions_active The number of active smtp-in sessions.\n")
	fmt.Fprintf(w, "# TYPE smtpd_sessions_active counter\n")
	fmt.Fprintf(w, "smtpd_sessions_active{direction=\"%s\"} %d\n", direction, m.sessionsActive)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP smtpd_sessions_total The number of active smtp-in sessions.\n")
	fmt.Fprintf(w, "# TYPE smtpd_sessions_total gauge\n")
	fmt.Fprintf(w, "smtpd_sessions_total{direction=\"%s\"} %d\n", direction, m.sessionsTotal)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP smtpd_sessions_inet4_active The number of active inet4 smtp-in sessions.\n")
	fmt.Fprintf(w, "# TYPE smtpd_sessions_inet4_active counter\n")
	fmt.Fprintf(w, "smtpd_sessions_inet4_active{direction=\"%s\"} %d\n", direction, m.sessionsInet4Active)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP smtpd_sessions_inet4_total The number of active inet4 smtp-in sessions.\n")
	fmt.Fprintf(w, "# TYPE smtpd_sessions_inet4_total gauge\n")
	fmt.Fprintf(w, "smtpd_sessions_inet4_total{direction=\"%s\"} %d\n", direction, m.sessionsInet4Total)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP smtpd_sessions_inet6_active The number of active inet6 smtp-in sessions.\n")
	fmt.Fprintf(w, "# TYPE smtpd_sessions_inet6_active counter\n")
	fmt.Fprintf(w, "smtpd_sessions_inet6_active{direction=\"%s\"} %d\n", direction, m.sessionsInet6Active)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP smtpd_sessions_inet6_total The number of active inet6 smtp-in sessions.\n")
	fmt.Fprintf(w, "# TYPE smtpd_sessions_inet6_total gauge\n")
	fmt.Fprintf(w, "smtpd_sessions_inet6_total{direction=\"%s\"} %d\n", direction, m.sessionsInet6Total)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP smtpd_sessions_unix_active The number of active unix smtp-in sessions.\n")
	fmt.Fprintf(w, "# TYPE smtpd_sessions_unix_active counter\n")
	fmt.Fprintf(w, "smtpd_sessions_unix_active{direction=\"%s\"} %d\n", direction, m.sessionsUnixActive)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP smtpd_sessions_unix_total The number of active unix smtp-in sessions.\n")
	fmt.Fprintf(w, "# TYPE smtpd_sessions_unix_total gauge\n")
	fmt.Fprintf(w, "smtpd_sessions_unix_total{direction=\"%s\"} %d\n", direction, m.sessionsUnixTotal)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP smtpd_sessions_tls_active The number of active TLS smtp-in sessions.\n")
	fmt.Fprintf(w, "# TYPE smtpd_sessions_tls_active counter\n")
	fmt.Fprintf(w, "smtpd_sessions_tls_active{direction=\"%s\"} %d\n", direction, m.sessionsTLSActive)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP smtpd_sessions_tls_total The number of active unix smtp-in sessions.\n")
	fmt.Fprintf(w, "# TYPE smtpd_sessions_tls_total gauge\n")
	fmt.Fprintf(w, "smtpd_sessions_tls_total{direction=\"%s\"} %d\n", direction, m.sessionsTLSTotal)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP smtpd_sessions_auth_active The number of active unix smtp-in sessions.\n")
	fmt.Fprintf(w, "# TYPE smtpd_sessions_auth_active counter\n")
	fmt.Fprintf(w, "smtpd_sessions_auth_active{direction=\"%s\"} %d\n", direction, m.sessionsAuthActive)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP smtpd_sessions_auth_total The number of active unix smtp-in sessions.\n")
	fmt.Fprintf(w, "# TYPE smtpd_sessions_auth_total gauge\n")
	fmt.Fprintf(w, "smtpd_sessions_auth_total{direction=\"%s\"} %d\n", direction, m.sessionsAuthTotal)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP smtpd_sessions_auth_failures The number of active unix smtp-in sessions.\n")
	fmt.Fprintf(w, "# TYPE smtpd_sessions_auth_failures gauge\n")
	fmt.Fprintf(w, "smtpd_sessions_auth_failures{direction=\"%s\"} %d\n", direction, m.sessionsAuthFailures)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP smtpd_tx_active The number of active smtp-in transactions.\n")
	fmt.Fprintf(w, "# TYPE smtpd_tx_active counter\n")
	fmt.Fprintf(w, "smtpd_tx_active{direction=\"%s\"} %d\n", direction, m.txActive)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP smtpd_tx_total The number of total smtp-in transactions.\n")
	fmt.Fprintf(w, "# TYPE smtpd_tx_total gauge\n")
	fmt.Fprintf(w, "smtpd_tx_total{direction=\"%s\"} %d\n", direction, m.txTotal)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP smtpd_tx_commit_total The number of total committed smtp-in transactions.\n")
	fmt.Fprintf(w, "# TYPE smtpd_tx_commit_total gauge\n")
	fmt.Fprintf(w, "smtpd_tx_commit_total{direction=\"%s\"} %d\n", direction, m.txCommitTotal)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP smtpd_tx_rollback_total The number of total rollbacked smtp-in transactions.\n")
	fmt.Fprintf(w, "# TYPE smtpd_tx_tollback_total gauge\n")
	fmt.Fprintf(w, "smtpd_tx_rollback_total{direction=\"%s\"} %d\n", direction, m.txRollbackTotal)
	fmt.Fprintf(w, "\n")
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	smtpMetricsHandlers("smtp-in", w, r)
	smtpMetricsHandlers("smtp-out", w, r)
}

func main() {
	exporter = flag.String("exporter", "localhost:13742", "exporter host and port")
	flag.Parse()

	scanner := bufio.NewScanner(os.Stdin)

	skipConfig(scanner)

	filterInit()

	go func() {
		http.HandleFunc("/metrics", metricsHandler)
		log.Fatal(http.ListenAndServe(*exporter, nil))
	}()

	for {
		if !scanner.Scan() {
			os.Exit(0)
		}

		line := scanner.Text()
		atoms := strings.Split(line, "|")
		if len(atoms) < 6 {
			log.Fatalf("missing atoms: %s", line)
		}

		switch atoms[0] {
		case "report":
			trigger(reporters, atoms)
		default:
			log.Fatalf("invalid stream: %s", atoms[0])
		}
	}
}
