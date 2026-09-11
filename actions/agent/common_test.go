package agent_actions

import (
	"testing"
	"time"

	"github.com/onsi/gomega"
)

// Thursday 10 September 2026, 09:05 — the morning the wrong-year
// commitments were written on live.
var reference = time.Date(2026, time.September, 10, 9, 5, 0, 0, time.UTC)

func resolve(t *testing.T, expression string) time.Time {
	t.Helper()
	when, ok := ResolveWhen(expression, reference)
	gomega.Expect(ok).To(gomega.BeTrue(), "could not read %q", expression)
	return when
}

func TestDurationsResolveFromNow(t *testing.T) {
	gomega.RegisterTestingT(t)

	gomega.Expect(resolve(t, "in 72 hours")).To(gomega.Equal(reference.Add(72 * time.Hour)))
	gomega.Expect(resolve(t, "72 hours")).To(gomega.Equal(reference.Add(72 * time.Hour)))
	gomega.Expect(resolve(t, "30m")).To(gomega.Equal(reference.Add(30 * time.Minute)))
	gomega.Expect(resolve(t, "in 30 minutes")).To(gomega.Equal(reference.Add(30 * time.Minute)))
	gomega.Expect(resolve(t, "in an hour")).To(gomega.Equal(reference.Add(time.Hour)))
	gomega.Expect(resolve(t, "in 2 weeks")).To(gomega.Equal(reference.Add(14 * 24 * time.Hour)))
}

func TestTheSeventyTwoHourPromiseLandsOnSunday(t *testing.T) {
	gomega.RegisterTestingT(t)

	when := resolve(t, "in 72 hours")

	gomega.Expect(when.Weekday()).To(gomega.Equal(time.Sunday))
	gomega.Expect(when.Format("2006-01-02")).To(gomega.Equal("2026-09-13"))
}

func TestRelativeDaysResolve(t *testing.T) {
	gomega.RegisterTestingT(t)

	gomega.Expect(resolve(t, "tomorrow").Format("2006-01-02 15:04")).To(gomega.Equal("2026-09-11 09:05"))
	gomega.Expect(resolve(t, "tomorrow at 9am").Format("2006-01-02 15:04")).To(gomega.Equal("2026-09-11 09:00"))
	gomega.Expect(resolve(t, "tomorrow at 14:30").Format("2006-01-02 15:04")).To(gomega.Equal("2026-09-11 14:30"))
	gomega.Expect(resolve(t, "next monday").Format("2006-01-02 15:04")).To(gomega.Equal("2026-09-14 09:00"))
	gomega.Expect(resolve(t, "next thursday").Format("2006-01-02")).To(gomega.Equal("2026-09-17"),
		"the same weekday means a week away, not today")
	gomega.Expect(resolve(t, "friday at 5pm").Format("2006-01-02 15:04")).To(gomega.Equal("2026-09-11 17:00"))
}

func TestCalendarDatesResolve(t *testing.T) {
	gomega.RegisterTestingT(t)

	for _, expression := range []string{
		"1 october", "1st october", "on 1st October", "October 1", "october 1st", "the 1st of october",
	} {
		gomega.Expect(resolve(t, expression).Format("2006-01-02 15:04")).To(
			gomega.Equal("2026-10-01 09:00"), "for %q", expression)
	}

	gomega.Expect(resolve(t, "1 october at 14:30").Format("2006-01-02 15:04")).To(gomega.Equal("2026-10-01 14:30"))
	gomega.Expect(resolve(t, "25 dec at 9am").Format("2006-01-02 15:04")).To(gomega.Equal("2026-12-25 09:00"))
	gomega.Expect(resolve(t, "2026-10-01").Format("2006-01-02 15:04")).To(gomega.Equal("2026-10-01 09:00"))
	gomega.Expect(resolve(t, "2027-03-09T08:30:00Z").Format("2006-01-02 15:04")).To(gomega.Equal("2027-03-09 08:30"))
}

func TestADateWithNoYearNeverResolvesIntoThePast(t *testing.T) {
	gomega.RegisterTestingT(t)

	// January has already gone in the reference year, so it means next.
	gomega.Expect(resolve(t, "14 january").Format("2006-01-02")).To(gomega.Equal("2027-01-14"))
	gomega.Expect(resolve(t, "1 september").Format("2006-01-02")).To(gomega.Equal("2027-09-01"))

	// Today's own date still means today, not a year away.
	gomega.Expect(resolve(t, "10 september at 18:00").Format("2006-01-02 15:04")).To(gomega.Equal("2026-09-10 18:00"))
}

func TestAnExplicitYearIsHonoured(t *testing.T) {
	gomega.RegisterTestingT(t)

	gomega.Expect(resolve(t, "1 october 2028").Format("2006-01-02")).To(gomega.Equal("2028-10-01"))
}

func TestABareDayOfMonthMeansTheNextOne(t *testing.T) {
	gomega.RegisterTestingT(t)

	gomega.Expect(resolve(t, "the 14th").Format("2006-01-02")).To(gomega.Equal("2026-09-14"))
	gomega.Expect(resolve(t, "the 1st").Format("2006-01-02")).To(gomega.Equal("2026-10-01"),
		"the 1st has passed this month, so it means next month")
}

func TestUnreadableExpressionsAreRefusedRatherThanGuessed(t *testing.T) {
	gomega.RegisterTestingT(t)

	for _, expression := range []string{
		"", "soon", "later", "when you get a chance", "after the sprint", "0 hours", "in the morning sometime",
	} {
		_, ok := ResolveWhen(expression, reference)
		gomega.Expect(ok).To(gomega.BeFalse(), "should not have read %q", expression)
	}
}

func TestEverythingItReadsIsInTheFuture(t *testing.T) {
	gomega.RegisterTestingT(t)

	for _, expression := range []string{
		"in 72 hours", "tomorrow", "tomorrow at 9am", "next monday", "1 october", "14 january",
		"the 14th", "the 1st", "25 december", "in 30 minutes", "friday at 5pm", "in a fortnight",
	} {
		when := resolve(t, expression)
		gomega.Expect(when.After(reference)).To(gomega.BeTrue(),
			"%q resolved to %s, which is in the past", expression, when)
	}
}

func TestRFC3339FormIsUTC(t *testing.T) {
	gomega.RegisterTestingT(t)

	gomega.Expect(ResolveWhenRFC3339("in 72 hours", reference)).To(gomega.Equal("2026-09-13T09:05:00Z"))
	gomega.Expect(ResolveWhenRFC3339("nonsense", reference)).To(gomega.BeEmpty())
}
