package hashpath

import "testing"

const emptySum = "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e"

func TestSumHex(t *testing.T) {
	if got := SumHex(nil); got != emptySum {
		t.Fatalf("SumHex(nil) = %s, want %s", got, emptySum)
	}
}

func TestKeyFromHex(t *testing.T) {
	want := "cf83/e135/" + emptySum
	if got := KeyFromHex(emptySum); got != want {
		t.Fatalf("KeyFromHex = %s, want %s", got, want)
	}
}

func TestKey(t *testing.T) {
	want := "cf83/e135/" + emptySum
	if got := Key(nil); got != want {
		t.Fatalf("Key(nil) = %s, want %s", got, want)
	}
}

func TestKeyFromHexPanicsOnShort(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a short-term panic was expected")
		}
	}()
	KeyFromHex("abc")
}
