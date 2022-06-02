package sign

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
)

type AwsSigner struct {
	Service string
	Region  string
}

func (s AwsSigner) SignRequest(req *http.Request) error {
	credentials := credentials.NewEnvCredentials()
	signer := v4.NewSigner(credentials)
	var b []byte
	if req.Body == nil {
		b = make([]byte, 0)
	} else {
		b2, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return err
		} else {
			b = b2
		}
	}
	body := bytes.NewReader(b)
	/*hash := sha256.New()
	var hb []byte
	hash.Write(hb)
	req.Header.Add("X-Amz-Content-Sha256", string(hb))*/
	_, err := signer.Sign(req, body, s.Service, s.Region, time.Now())
	if err != nil {
		return err
	}
	return nil
}
