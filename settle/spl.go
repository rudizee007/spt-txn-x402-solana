package settle

import (
	"encoding/binary"
	"encoding/hex"
)

// mustHex32 decodes a 32-byte hex constant at init; a bad constant is a
// programming error, not a runtime condition.
func mustHex32(h string) [32]byte {
	var a [32]byte
	b, err := hex.DecodeString(h)
	if err != nil || len(b) != 32 {
		panic("settle: bad 32-byte hex constant: " + h)
	}
	copy(a[:], b)
	return a
}

// Well-known Solana program ids (base58 names shown; bytes are the address).
// Mainnet and devnet share these addresses.
var (
	// SPL Token program — TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA
	SPLTokenProgramID = mustHex32("06ddf6e1d765a193d9cbe146ceeb79ac1cb485ed5f5b37913a8cf5857eff00a9")
	// Token-2022 program — TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb
	Token2022ProgramID = mustHex32("06ddf6e1ee758fde18425dbce46ccddab61afc4d83b90d27febdf928d8a18bfc")

	// USDCDevnetMint is Circle's USDC mint on Solana devnet —
	// 4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU (faucet: https://faucet.circle.com).
	USDCDevnetMint = mustHex32("3b442cb3912157f13a933d0134282d032b5ffecd01a2dbf1b7790608df002ea7")
)

// USDCDecimals is the decimal precision of USDC (micro-USDC base units).
const USDCDecimals uint8 = 6

// BuildTransferChecked constructs a real SPL Token (or Token-2022, if you pass
// its program id) TransferChecked instruction — the actual instruction that
// moves value on devnet/mainnet. Data layout is [12][amount u64 LE][decimals u8];
// accounts are [source, mint, destination, authority]. This is exactly what
// AssertMatches / AssertTransactionPays inspects before signing, so the guard
// runs against the real transfer, not a stand-in.
func BuildTransferChecked(programID, source, mint, destination, authority [32]byte, amount uint64, decimals uint8) Instruction {
	data := make([]byte, 10)
	data[0] = TransferCheckedDiscriminator
	binary.LittleEndian.PutUint64(data[1:9], amount)
	data[9] = decimals
	return Instruction{
		ProgramID: programID,
		Data:      data,
		Accounts:  [][32]byte{source, mint, destination, authority},
	}
}
