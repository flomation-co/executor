package add_to_conversation

import (
	"testing"

	. "github.com/onsi/gomega"
)

// A Slack thread id or Telegram chat id passed straight through from trigger
// data reaches the API as a malformed UUID, and the request fails with
// "invalid input syntax for type uuid". On live that was happening against ids
// like "VijsyshjBd", silently costing those flows their history — the failure
// is a 500 in a log, not something the flow author ever sees.
func TestIsUUID_TellsAPlatformIdFromAChannelOne(t *testing.T) {
	RegisterTestingT(t)

	// Platform conversation ids.
	Expect(isUUID("11111111-2222-3333-4444-555555555555")).To(BeTrue())
	Expect(isUUID("A1B2C3D4-E5F6-7890-ABCD-EF1234567890")).To(BeTrue(), "case should not matter")

	// The ones that were actually arriving.
	Expect(isUUID("VijsyshjBd")).To(BeFalse(), "a Slack-style id")
	Expect(isUUID("")).To(BeFalse())
	Expect(isUUID("-1002345678901")).To(BeFalse(), "a Telegram chat id")
	Expect(isUUID("C06DGV40NH0")).To(BeFalse(), "a Slack channel id")

	// Near misses, since a shape check is only useful if it is strict.
	Expect(isUUID("11111111-2222-3333-4444-5555555555555")).To(BeFalse(), "too long")
	Expect(isUUID("11111111-2222-3333-4444-55555555555")).To(BeFalse(), "too short")
	Expect(isUUID("11111111222233334444555555555555")).To(BeFalse(), "no separators")
	Expect(isUUID("11111111-2222-3333-4444-55555555555g")).To(BeFalse(), "not hex")
	Expect(isUUID("11111111x2222-3333-4444-555555555555")).To(BeFalse(), "separator in the wrong place")
}
