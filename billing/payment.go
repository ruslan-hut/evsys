package billing

import (
	"evsys/entity"
	"evsys/internal"
	"evsys/internal/config"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type Payment struct {
	database Database
	logger   internal.LogHandler
	apiUrl   string
	apiKey   string
	mutex    *sync.Mutex
}

func NewPaymentService(conf *config.Config) *Payment {
	payment := &Payment{
		apiUrl: conf.Payment.ApiUrl,
		apiKey: conf.Payment.ApiKey,
		mutex:  &sync.Mutex{},
	}
	payment.Start()
	return payment
}

func (p *Payment) SetDatabase(database Database) {
	p.database = database
}

func (p *Payment) SetLogger(logger internal.LogHandler) {
	p.logger = logger
}

func (p *Payment) TransactionPayment(transaction *entity.Transaction) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	requestUrl := fmt.Sprintf("%s/pay/%d", p.apiUrl, transaction.Id)

	req, err := http.NewRequest("GET", requestUrl, nil)
	if err != nil {
		p.logger.Error("payment: create request", err)
		return
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		p.logger.Error("payment: send request", err)
		return
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			p.logger.Error("payment: close response body", err)
		}
	}(resp.Body)

	// analise response status
	if resp.StatusCode != http.StatusOK {
		p.logger.Warn(fmt.Sprintf("payment: transaction %v response status: %v", transaction.Id, resp.StatusCode))
		return
	}

}

func (p *Payment) checkTransactions() {
	if p.database == nil {
		return
	}
	transactions, err := p.database.GetNotBilledTransactions()
	if err != nil {
		p.logger.Error("payment: get not billed transactions", err)
		return
	}
	for _, transaction := range transactions {
		p.TransactionPayment(transaction)
	}
}

// Start ticker with check transactions
func (p *Payment) Start() {
	ticker := time.NewTicker(3 * time.Minute)

	go func() {
		for range ticker.C {
			p.checkTransactions()
		}
	}()
}
