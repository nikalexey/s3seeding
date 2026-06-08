package seed

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"log"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"s3bench/internal/config"
	"s3bench/internal/hashpath"
	"s3bench/internal/s3client"
)

type SizeRange struct {
	Min int64
	Max int64
}

type Distribution struct {
	SmallPct  float64
	MediumPct float64
	LargePct  float64
	Small     SizeRange
	Medium    SizeRange
	Large     SizeRange
}

// FromConfig builds a Distribution from the seed configuration, converting the
// kilobyte size ranges into bytes.
func FromConfig(s config.Seed) Distribution {
	const kb = 1024
	return Distribution{
		SmallPct:  s.SmallPct,
		MediumPct: s.MediumPct,
		LargePct:  s.LargePct,
		Small:     SizeRange{Min: s.Small.MinKB * kb, Max: s.Small.MaxKB * kb},
		Medium:    SizeRange{Min: s.Medium.MinKB * kb, Max: s.Medium.MaxKB * kb},
		Large:     SizeRange{Min: s.Large.MinKB * kb, Max: s.Large.MaxKB * kb},
	}
}

type Options struct {
	Workers    int
	MaxObjects int64
	MaxBytes   int64
	Dist       Distribution
}

type Seeder struct {
	client *s3client.Client
	opt    Options
	cancel context.CancelFunc

	objects atomic.Int64
	bytes   atomic.Int64
	small   atomic.Int64
	medium  atomic.Int64
	large   atomic.Int64
	errors  atomic.Int64
}

func New(client *s3client.Client, opt Options) *Seeder {
	if opt.Workers <= 0 {
		opt.Workers = 8
	}
	return &Seeder{client: client, opt: opt}
}

func (s *Seeder) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.cancel = cancel

	start := time.Now()
	stopTick := make(chan struct{})
	go s.progress(start, stopTick)

	var wg sync.WaitGroup
	for range s.opt.Workers {
		wg.Go(func() { s.worker(ctx) })
	}
	wg.Wait()

	close(stopTick)
	s.report(start, "TOTAL seeded")
}

func (s *Seeder) worker(ctx context.Context) {
	r := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	mediumMax := s.opt.Dist.Medium.Max
	var reuse []byte // reusable buffer for small/medium objects (up to mediumMax)

	for {
		if ctx.Err() != nil || s.limitReached() {
			return
		}

		class, size := s.pick(r)

		var b []byte
		if size <= mediumMax {
			if int64(cap(reuse)) < size {
				reuse = make([]byte, mediumMax)
			}
			b = reuse[:size]
		} else {
			b = make([]byte, size)
		}
		fillRandom(r, b)

		sum := sha512.Sum512(b)
		key := hashpath.KeyFromHex(hex.EncodeToString(sum[:]))

		if err := s.client.Put(ctx, key, bytes.NewReader(b), size); err != nil {
			if ctx.Err() != nil {
				return
			}
			n := s.errors.Add(1)
			log.Printf("upload error: %v", err)
			if n >= 20 && s.objects.Load() == 0 {
				log.Printf("20 errors without a single successful upload — stopping (check access/bucket)")
				s.cancel()
				return
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		s.objects.Add(1)
		s.bytes.Add(size)
		switch class {
		case "small":
			s.small.Add(1)
		case "medium":
			s.medium.Add(1)
		case "large":
			s.large.Add(1)
		}
	}
}

func (s *Seeder) pick(r *rand.Rand) (string, int64) {
	d := s.opt.Dist
	x := r.Float64() * 100
	switch {
	case x < d.SmallPct:
		return "small", randSize(r, d.Small)
	case x < d.SmallPct+d.MediumPct:
		return "medium", randSize(r, d.Medium)
	default:
		return "large", randSize(r, d.Large)
	}
}

func randSize(r *rand.Rand, sr SizeRange) int64 {
	if sr.Max <= sr.Min {
		return sr.Min
	}
	return sr.Min + r.Int64N(sr.Max-sr.Min+1)
}

func (s *Seeder) limitReached() bool {
	if s.opt.MaxObjects > 0 && s.objects.Load() >= s.opt.MaxObjects {
		return true
	}
	if s.opt.MaxBytes > 0 && s.bytes.Load() >= s.opt.MaxBytes {
		return true
	}
	return false
}

func (s *Seeder) progress(start time.Time, stop <-chan struct{}) {
	const interval = 5 * time.Second
	t := time.NewTicker(interval)
	defer t.Stop()

	prevTime := start
	var prevObj, prevBy int64
	for {
		select {
		case <-stop:
			return
		case now := <-t.C:
			obj := s.objects.Load()
			by := s.bytes.Load()
			el := now.Sub(prevTime).Seconds()
			if el <= 0 {
				el = interval.Seconds()
			}
			objRate := float64(obj-prevObj) / el
			byRate := float64(by-prevBy) / (1 << 20) / el
			log.Printf("seeding: objects=%d (small=%d medium=%d large=%d) data=%.2f GB  %.0f obj/s  %.1f MB/s  errors=%d",
				obj, s.small.Load(), s.medium.Load(), s.large.Load(),
				float64(by)/(1<<30), objRate, byRate, s.errors.Load())
			prevTime, prevObj, prevBy = now, obj, by
		}
	}
}

func (s *Seeder) report(start time.Time, prefix string) {
	el := time.Since(start).Seconds()
	if el <= 0 {
		el = 1
	}
	obj := s.objects.Load()
	by := s.bytes.Load()
	log.Printf("%s: objects=%d (small=%d medium=%d large=%d) data=%.2f GB  %.0f obj/s  %.1f MB/s  errors=%d",
		prefix, obj, s.small.Load(), s.medium.Load(), s.large.Load(),
		float64(by)/(1<<30), float64(obj)/el, float64(by)/(1<<20)/el, s.errors.Load())
}

func fillRandom(r *rand.Rand, buf []byte) {
	i := 0
	for ; i+8 <= len(buf); i += 8 {
		binary.LittleEndian.PutUint64(buf[i:], r.Uint64())
	}
	if i < len(buf) {
		var tmp [8]byte
		binary.LittleEndian.PutUint64(tmp[:], r.Uint64())
		copy(buf[i:], tmp[:])
	}
}
