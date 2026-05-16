package email

import "fmt"

// PasswordResetMessage returns the email sent when a user requests a password reset.
func PasswordResetMessage(to, resetURL string) Message {
	return Message{
		To:      to,
		Subject: "Reset your Circl password",
		HTML: fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;max-width:480px;margin:0 auto;padding:24px">
  <h2>Reset your password</h2>
  <p>Click the button below to set a new password. This link expires in <strong>1 hour</strong>.</p>
  <a href="%s" style="display:inline-block;padding:12px 24px;background:#6366f1;color:#fff;border-radius:6px;text-decoration:none;font-weight:600">Reset password</a>
  <p style="margin-top:24px;color:#666;font-size:13px">If you didn't request this, you can safely ignore this email.</p>
</body>
</html>`, resetURL),
		Text: fmt.Sprintf("Reset your Circl password\n\n%s\n\nThis link expires in 1 hour. If you didn't request this, ignore this email.", resetURL),
	}
}

// AccountDeletionMessage returns the email sent when a user soft-deletes their account.
func AccountDeletionMessage(to, loginURL string) Message {
	return Message{
		To:      to,
		Subject: "Your Circl account has been scheduled for deletion",
		HTML: fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;max-width:480px;margin:0 auto;padding:24px">
  <h2>Account deletion scheduled</h2>
  <p>We received a request to delete your Circl account. Your account and all associated data will be <strong>permanently deleted in 30 days</strong>.</p>
  <p>Changed your mind? Simply sign in before the 30-day window closes and your account will be automatically restored.</p>
  <a href="%s" style="display:inline-block;padding:12px 24px;background:#6366f1;color:#fff;border-radius:6px;text-decoration:none;font-weight:600">Sign in to reactivate</a>
  <p style="margin-top:24px;color:#666;font-size:13px">If you did not request this deletion, sign in immediately to reactivate your account.</p>
</body>
</html>`, loginURL),
		Text: fmt.Sprintf("Your Circl account has been scheduled for deletion.\n\nYour account will be permanently deleted in 30 days. To cancel, sign in before the deadline:\n\n%s\n\nIf you did not request this, sign in immediately to reactivate your account.", loginURL),
	}
}

// AppealMessage returns the email sent when a user is suspended or banned.
// The appeal link routes to /appeal/{token} on the frontend, which the
// locked-out user can reach without logging in.
func AppealMessage(to, appealURL, reason string, permanent bool) Message {
	subject := "Your Circl account has been suspended"
	headline := "Your account has been suspended"
	intro := "Your Circl account has been suspended following a review of activity that may violate our community guidelines."
	if permanent {
		subject = "Your Circl account has been banned"
		headline = "Your account has been banned"
		intro = "Your Circl account has been permanently banned following a review of activity that violates our community guidelines."
	}
	reasonBlock := ""
	if reason != "" {
		reasonBlock = fmt.Sprintf(`<p style="color:#666;font-size:13px"><strong>Reason given:</strong> %s</p>`, reason)
	}
	return Message{
		To:      to,
		Subject: subject,
		HTML: fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;max-width:480px;margin:0 auto;padding:24px">
  <h2>%s</h2>
  <p>%s</p>
  %s
  <p>If you believe this is a mistake, you can submit an appeal. Our team reviews every appeal individually. This link expires in <strong>30 days</strong>.</p>
  <a href="%s" style="display:inline-block;padding:12px 24px;background:#6366f1;color:#fff;border-radius:6px;text-decoration:none;font-weight:600">Submit an appeal</a>
  <p style="margin-top:24px;color:#666;font-size:13px">If you have questions, contact us at info.circl.ar@gmail.com.</p>
</body>
</html>`, headline, intro, reasonBlock, appealURL),
		Text: fmt.Sprintf("%s\n\n%s\n\nIf you believe this is a mistake, submit an appeal here (link expires in 30 days):\n\n%s\n\nQuestions: info.circl.ar@gmail.com", headline, intro, appealURL),
	}
}

// AppealResolutionMessage returns the email sent once admin approves or
// denies an appeal. Approved appeals reactivate the account; denied appeals
// leave the suspension in place.
func AppealResolutionMessage(to, loginURL, note string, approved bool) Message {
	subject := "Update on your Circl appeal"
	headline := "Your appeal was approved"
	body := "We have reviewed your appeal and reactivated your account. You can log in again now."
	cta := fmt.Sprintf(`<a href="%s" style="display:inline-block;padding:12px 24px;background:#6366f1;color:#fff;border-radius:6px;text-decoration:none;font-weight:600">Sign in</a>`, loginURL)
	if !approved {
		headline = "Your appeal was denied"
		body = "We have reviewed your appeal and our decision stands."
		cta = ""
	}
	noteBlock := ""
	if note != "" {
		noteBlock = fmt.Sprintf(`<p style="color:#444"><strong>Note from the review team:</strong> %s</p>`, note)
	}
	return Message{
		To:      to,
		Subject: subject,
		HTML: fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;max-width:480px;margin:0 auto;padding:24px">
  <h2>%s</h2>
  <p>%s</p>
  %s
  %s
  <p style="margin-top:24px;color:#666;font-size:13px">If you have questions, contact us at info.circl.ar@gmail.com.</p>
</body>
</html>`, headline, body, noteBlock, cta),
		Text: fmt.Sprintf("%s\n\n%s\n\n%s\n\nQuestions: info.circl.ar@gmail.com", headline, body, note),
	}
}

// EmailVerificationMessage returns the email sent to verify a new account.
func EmailVerificationMessage(to, verifyURL string) Message {
	return Message{
		To:      to,
		Subject: "Verify your Circl email address",
		HTML: fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;max-width:480px;margin:0 auto;padding:24px">
  <h2>Verify your email</h2>
  <p>Click the button below to verify your email address and activate your account. This link expires in <strong>24 hours</strong>.</p>
  <a href="%s" style="display:inline-block;padding:12px 24px;background:#6366f1;color:#fff;border-radius:6px;text-decoration:none;font-weight:600">Verify email</a>
</body>
</html>`, verifyURL),
		Text: fmt.Sprintf("Verify your Circl email address\n\n%s\n\nThis link expires in 24 hours.", verifyURL),
	}
}
