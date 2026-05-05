package messaging

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type KafkaReader struct {
	reader *kafka.Reader
}

func NewKafkaReader(reader *kafka.Reader) *KafkaReader {
	return &KafkaReader{reader: reader}
}

func (r *KafkaReader) Read(ctx context.Context) ([]byte, error) {
	msg, err := r.reader.ReadMessage(ctx)
	if err != nil {
		return nil, err
	}
	return msg.Value, nil
}
