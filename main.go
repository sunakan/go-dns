package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"slices"
	"sync"
	"syscall"

	"github.com/miekg/dns"
)

var (
	defaultSubdomains = []string{
		"t.example.com.",
		"ns1.t.example.com.",
		"www.t.example.com.",
		"test001.t.example.com.",
	}
	subdomains   = defaultSubdomains
	muSubdomains = sync.RWMutex{}
	rrCache      = sync.Map{}
)

func resetSubdomains() {
	muSubdomains.Lock()
	defer muSubdomains.Unlock()
	subdomains = defaultSubdomains
}

// dns.RR はインターフェース
// DNSリソースレコードの操作である、シリアライズ、デシリアライズ、比較などの機能を提供
// 中のメソッドを実装するとこで、様々なタイプDNSレコードを（A,AAAA,MX,CNAMEなど)を統一的に扱うことが可能
func newRR(s string) dns.RR {
	if rr, ok := rrCache.Load(s); ok {
		return rr.(dns.RR)
	}
	r, _ := dns.NewRR(s)
	rrCache.Store(s, r)
	return r
}

// w: DNSレスポンスを書き込みするためのインターフェース
// r: 受信したDNSクエリメッセージ
func handle(w dns.ResponseWriter, r *dns.Msg) {
	// レスポンスの準備
	// 新しいDNSメッセージを作成し、受信したクエリに対する返信として設定
	m := new(dns.Msg)
	m.SetReply(r)

	// "t.example.com." ドメインに対するNSクエリの場合、特定のNSレコードとAレコードを返す
	// NSクエリの発行は、dig @*.*.*.* -p 50053 t.example.com NS +short
	if r.Question[0].Qtype == dns.TypeNS && r.Question[0].Name == "t.example.com." {
		m.Answer = []dns.RR{
			newRR("t.example.com. 5 IN NS ns1.t.example.com."),
		}
		m.Extra = []dns.RR{
			newRR("ns1.t.example.com. 5 IN A 192.168.0.11"),
		}
	} else {
		muSubdomains.RLock()
		defer muSubdomains.RUnlock()

		// subdomainsに含まれているならば、Aレコードを返す
		if slices.Contains(subdomains, r.Question[0].Name) {
			m.Answer = []dns.RR{
				newRR(r.Question[0].Name + " 5 IN A 192.168.0.11"),
			}
		} else {
			// ここを返さないことで、水責めに対して
			return
		}
	}
	w.WriteMsg(m)
}

// 指定ネットワークでDNSサーバー処理を実行
func serveDNS(server *dns.Server) {
	if err := server.ListenAndServe(); err != nil {
		log.Fatal("Failed to start server: ", err)
	}
}

func main() {
	resetSubdomains()
	dns.HandleFunc("t.example.com.", handle)

	udpSrv := &dns.Server{Addr: ":50053", Net: "udp"}
	defer udpSrv.Shutdown()
	go serveDNS(udpSrv)

	fmt.Println("_________________________________________mydns")
	fmt.Println(" __  __  __   __   ____   _   _   ____  ")
	fmt.Println("|  \\/  | \\ \\ / /  |  _ \\ | \\ | | / ___| ")
	fmt.Println("| |\\/| |  \\ V /   | | | ||  \\| | \\___ \\ ")
	fmt.Println("| |  | |   | |    | |_| || |\\  |  ___) |")
	fmt.Println("|_|  |_|   |_|    |____/ |_| \\_| |____/ ")
	fmt.Println("_________________________________________mydns")

	// シグナルを受信したら終了
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit
	fmt.Println("Hello-4")
}
