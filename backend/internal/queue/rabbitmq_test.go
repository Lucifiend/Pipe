package queue

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"encoding/json"
	"time"
)

func getRabbitMQURL() string {
	if url := os.Getenv("RABBITMQ_URL"); url != "" {
		return url
	}
	return "amqp://guest:guest@127.0.0.1:5672/"
}


func TestRabbitMQ_PublishAndConsume(t *testing.T) {
	url := getRabbitMQURL()
	rmq, err := NewRabbitmq(url)
	if err != nil {
		t.Skipf("Skipping test: RabbitMQ connection failed at %s (%v)", url, err)
	}
	defer rmq.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	targetID := int64(998811)
	received := make(chan int64, 1)

	go func() {
		_ = rmq.ConsumerPipeline(ctx, func(cCtx context.Context, msg PipelineMessage) error {
			received <- msg.PipelineId
			cancel() 
			return nil
		})
	}()

	// Small pause to ensure consumer is bound
	time.Sleep(100 * time.Millisecond)

	if err := rmq.PublishPipeline(ctx, targetID); err != nil {
		t.Fatalf("failed to publish message: %v", err)
	}

	select {
	case id := <-received:
		if id != targetID {
			t.Errorf("expected pipeline ID %d, got %d", targetID, id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for pipeline message")
	}
}

func BenchmarkRabbitMQ_Publish(b *testing.B) {
	url := getRabbitMQURL()
	rmq, err := NewRabbitmq(url)
	if err != nil {
		b.Skipf("Skipping benchmark: RabbitMQ connection failed at %s (%v)", url, err)
	}
	defer rmq.Close()

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := rmq.PublishPipeline(ctx, int64(i))
		if err != nil {
			b.Fatalf("failed to publish message at iteration %d: %v", i, err)
		}
	}
}

func BenchmarkRabbitMQ_EndToEndPipeline(b *testing.B) {
	url := getRabbitMQURL()
	rmq, err := NewRabbitmq(url)
	if err != nil {
		b.Skipf("Skipping benchmark: RabbitMQ connection failed at %s (%v)", url, err)
	}
	defer rmq.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var consumedCount int64

	go func() {
		_ = rmq.ConsumerPipeline(ctx, func(cCtx context.Context, msg PipelineMessage) error {
			atomic.AddInt64(&consumedCount, 1)
			return nil
		})
	}()

	time.Sleep(100 * time.Millisecond)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := rmq.PublishPipeline(ctx, int64(i)); err != nil {
			b.Fatalf("publish failed: %v", err)
		}
	}

	for atomic.LoadInt64(&consumedCount) < int64(b.N) {
		time.Sleep(1 * time.Millisecond)
	}
}

func BenchmarkPipelineMessage_JSONMarshal(b *testing.B) {
	msg := PipelineMessage{PipelineId: 123456789}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(msg)
		if err != nil {
			b.Fatal(err)
		}
	}
}
