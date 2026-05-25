package realtime

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

type RedisOptions struct {
	Addr           string
	Password       string
	DB             int
	MaxMemory      string
	EvictionPolicy string
	Timeout        time.Duration
}

type RedisStore struct {
	options RedisOptions
}

func NewRedisStore(options RedisOptions) *RedisStore {
	if options.Timeout <= 0 {
		options.Timeout = 2 * time.Second
	}
	return &RedisStore{options: options}
}

func (s *RedisStore) SetRaw(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	args := [][]byte{[]byte("SET"), []byte(key), value}
	if ttl > 0 {
		seconds := int64(ttl.Round(time.Second).Seconds())
		if seconds < 1 {
			seconds = 1
		}
		args = append(args, []byte("EX"), []byte(strconv.FormatInt(seconds, 10)))
	}

	_, err := s.command(ctx, args...)
	return err
}

func (s *RedisStore) GetRaw(ctx context.Context, key string) ([]byte, bool, error) {
	reply, err := s.command(ctx, []byte("GET"), []byte(key))
	if err != nil {
		return nil, false, err
	}
	if reply.nil {
		return nil, false, nil
	}
	return append([]byte(nil), reply.data...), true, nil
}

func (s *RedisStore) ApplyMemoryPolicy(ctx context.Context) error {
	if strings.TrimSpace(s.options.MaxMemory) != "" {
		if _, err := s.command(ctx, []byte("CONFIG"), []byte("SET"), []byte("maxmemory"), []byte(strings.TrimSpace(s.options.MaxMemory))); err != nil {
			return err
		}
	}
	if strings.TrimSpace(s.options.EvictionPolicy) != "" {
		if _, err := s.command(ctx, []byte("CONFIG"), []byte("SET"), []byte("maxmemory-policy"), []byte(strings.TrimSpace(s.options.EvictionPolicy))); err != nil {
			return err
		}
	}
	return nil
}

func (s *RedisStore) command(ctx context.Context, args ...[]byte) (redisReply, error) {
	if strings.TrimSpace(s.options.Addr) == "" {
		return redisReply{}, fmt.Errorf("redis addr is required")
	}

	dialer := net.Dialer{Timeout: s.options.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", s.options.Addr)
	if err != nil {
		return redisReply{}, fmt.Errorf("dial redis: %w", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(s.options.Timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)

	reader := bufio.NewReader(conn)
	if strings.TrimSpace(s.options.Password) != "" {
		if err := writeRESP(conn, [][]byte{[]byte("AUTH"), []byte(s.options.Password)}); err != nil {
			return redisReply{}, err
		}
		if _, err := readRESP(reader); err != nil {
			return redisReply{}, err
		}
	}
	if s.options.DB > 0 {
		if err := writeRESP(conn, [][]byte{[]byte("SELECT"), []byte(strconv.Itoa(s.options.DB))}); err != nil {
			return redisReply{}, err
		}
		if _, err := readRESP(reader); err != nil {
			return redisReply{}, err
		}
	}

	if err := writeRESP(conn, args); err != nil {
		return redisReply{}, err
	}
	return readRESP(reader)
}

func writeRESP(conn net.Conn, args [][]byte) error {
	var buffer bytes.Buffer
	buffer.WriteByte('*')
	buffer.WriteString(strconv.Itoa(len(args)))
	buffer.WriteString("\r\n")
	for _, arg := range args {
		buffer.WriteByte('$')
		buffer.WriteString(strconv.Itoa(len(arg)))
		buffer.WriteString("\r\n")
		buffer.Write(arg)
		buffer.WriteString("\r\n")
	}
	_, err := conn.Write(buffer.Bytes())
	return err
}

type redisReply struct {
	data    []byte
	integer int64
	array   []redisReply
	nil     bool
}

func readRESP(reader *bufio.Reader) (redisReply, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return redisReply{}, err
	}

	switch prefix {
	case '+':
		line, err := readLine(reader)
		return redisReply{data: []byte(line)}, err
	case '-':
		line, err := readLine(reader)
		if err != nil {
			return redisReply{}, err
		}
		return redisReply{}, fmt.Errorf("redis error: %s", line)
	case ':':
		line, err := readLine(reader)
		if err != nil {
			return redisReply{}, err
		}
		value, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return redisReply{}, err
		}
		return redisReply{integer: value}, nil
	case '$':
		line, err := readLine(reader)
		if err != nil {
			return redisReply{}, err
		}
		size, err := strconv.Atoi(line)
		if err != nil {
			return redisReply{}, err
		}
		if size < 0 {
			return redisReply{nil: true}, nil
		}
		value := make([]byte, size+2)
		if _, err := io.ReadFull(reader, value); err != nil {
			return redisReply{}, err
		}
		return redisReply{data: append([]byte(nil), value[:size]...)}, nil
	case '*':
		line, err := readLine(reader)
		if err != nil {
			return redisReply{}, err
		}
		size, err := strconv.Atoi(line)
		if err != nil {
			return redisReply{}, err
		}
		if size < 0 {
			return redisReply{nil: true}, nil
		}
		items := make([]redisReply, 0, size)
		for i := 0; i < size; i++ {
			item, err := readRESP(reader)
			if err != nil {
				return redisReply{}, err
			}
			items = append(items, item)
		}
		return redisReply{array: items}, nil
	default:
		return redisReply{}, fmt.Errorf("unsupported redis response prefix %q", prefix)
	}
}

func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}
