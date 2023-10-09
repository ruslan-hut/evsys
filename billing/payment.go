package billing

import (
	"evsys/internal"
	"evsys/internal/config"
	"evsys/models"
	"fmt"
	"io"
	"net/http"
)

type Payment struct {
	database internal.Database
	logger   internal.LogHandler
	apiUrl   string
	apiKey   string
}

func NewPaymentService(conf *config.Config) *Payment {
	return &Payment{
		apiUrl: conf.Payment.ApiUrl,
		apiKey: conf.Payment.ApiKey,
	}
}

func (p *Payment) SetDatabase(database internal.Database) {
	p.database = database
}

func (p *Payment) SetLogger(logger internal.LogHandler) {
	p.logger = logger
}

func (p *Payment) TransactionPayment(transaction *models.Transaction) {

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
		err := Body.Close()
		if err != nil {
			p.logger.Error("payment: close response body", err)
		}
	}(resp.Body)

	// analise response status
	if resp.StatusCode != http.StatusOK {
		p.logger.Warn(fmt.Sprintf("payment: response status: %v", resp.StatusCode))
		return
	}

}
