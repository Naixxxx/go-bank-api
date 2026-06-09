package integrations

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/beevik/etree"
	"github.com/sirupsen/logrus"
)

type CBRClient struct{ httpClient *http.Client }

func NewCBRClient() *CBRClient {
	return &CBRClient{httpClient: &http.Client{Timeout: 10 * time.Second}}
}

func (c *CBRClient) KeyRate() (float64, error) {
	body := c.buildSOAPRequest()

	req, err := http.NewRequest("POST", "https://www.cbr.ru/DailyInfoWebServ/DailyInfo.asmx", bytes.NewBufferString(body))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	req.Header.Set("SOAPAction", "http://web.cbr.ru/KeyRate")

	resp, err := c.httpClient.Do(req)

	if err != nil {
		return 0, fmt.Errorf("cbr request failed: %w", err)
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			logrus.WithError(err).Error("body close failure")
		}
	}()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("cbr status %d: %s", resp.StatusCode, string(raw))
	}

	return parseRate(raw)
}

func (c *CBRClient) buildSOAPRequest() string {
	from := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	to := time.Now().Format("2006-01-02")

	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<soap12:Envelope xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">
  <soap12:Body>
    <KeyRate xmlns="http://web.cbr.ru/">
      <fromDate>%s</fromDate>
      <ToDate>%s</ToDate>
    </KeyRate>
  </soap12:Body>
</soap12:Envelope>`, from, to)
}

func parseRate(raw []byte) (float64, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(raw); err != nil {
		return 0, err
	}

	elements := doc.FindElements("//diffgram/KeyRate/KR")
	if len(elements) == 0 {
		return 0, errors.New("key rate not found")
	}

	rateEl := elements[0].FindElement("./Rate")
	if rateEl == nil {
		return 0, errors.New("rate tag absent")
	}

	var rate float64
	if _, err := fmt.Sscanf(rateEl.Text(), "%f", &rate); err != nil {
		return 0, err
	}

	return rate, nil
}
