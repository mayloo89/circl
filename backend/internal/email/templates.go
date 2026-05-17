package email

import (
	"fmt"
	"time"
)

// wrap composes the shared HTML chrome for every transactional email — brand
// header with the logo, a bone-coloured card carrying the content, and a
// muted footer. `frontendURL` must be the absolute base URL of the frontend
// (no trailing slash) so the logo `<img src>` resolves cross-client.
//
// Inline styles only: most email clients strip <style> blocks and external
// stylesheets, so every visual choice has to live on the element itself.
func wrap(frontendURL, headline, bodyHTML string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="margin:0;padding:0;background:#0A0A0F;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#1F1B2D">
  <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="background:#0A0A0F;padding:32px 16px">
    <tr><td align="center">
      <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="520" style="max-width:520px;background:#F5F1EA;border-radius:12px;overflow:hidden">
        <tr><td style="padding:24px 28px;background:#0A0A0F;text-align:left">
          <img src="%s/branding/logo-email-dark.png" alt="Circl" width="120" style="display:block;border:0;outline:none;text-decoration:none;height:auto">
        </td></tr>
        <tr><td style="padding:28px">
          <h1 style="margin:0 0 12px;font-size:22px;line-height:1.3;color:#0A0A0F;font-weight:700">%s</h1>
          %s
        </td></tr>
        <tr><td style="padding:18px 28px;background:#EFEAE0;border-top:1px solid #E1DCD0;color:#666;font-size:12px;line-height:1.6">
          Circl · <a href="mailto:info.circl.ar@gmail.com" style="color:#D14A88;text-decoration:none">info.circl.ar@gmail.com</a>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, frontendURL, headline, bodyHTML)
}

// button renders an inline-block CTA in the brand magenta. Kept as a helper
// so all transactional CTAs share one source of truth and we never have to
// hunt for one stray "background:#6366f1" again.
func button(href, label string) string {
	return fmt.Sprintf(
		`<a href="%s" style="display:inline-block;padding:12px 24px;background:#D14A88;color:#fff;border-radius:6px;text-decoration:none;font-weight:600">%s</a>`,
		href, label)
}

// PasswordResetMessage returns the email sent when a user requests a password reset.
func PasswordResetMessage(frontendURL, to, resetURL string) Message {
	body := fmt.Sprintf(`
		<p style="margin:0 0 16px;color:#1F1B2D;font-size:15px;line-height:1.6">Click the button below to set a new password. This link expires in <strong>1 hour</strong>.</p>
		%s
		<p style="margin:24px 0 0;color:#666;font-size:13px;line-height:1.6">If you didn't request this, you can safely ignore this email.</p>`,
		button(resetURL, "Reset password"))
	return Message{
		To:      to,
		Subject: "Reset your Circl password",
		HTML:    wrap(frontendURL, "Reset your password", body),
		Text:    fmt.Sprintf("Reset your Circl password\n\n%s\n\nThis link expires in 1 hour. If you didn't request this, ignore this email.", resetURL),
	}
}

// AccountDeletionMessage returns the email sent when a user soft-deletes their account.
func AccountDeletionMessage(frontendURL, to, loginURL string) Message {
	body := fmt.Sprintf(`
		<p style="margin:0 0 16px;color:#1F1B2D;font-size:15px;line-height:1.6">We received a request to delete your Circl account. Your account and all associated data will be <strong>permanently deleted in 30 days</strong>.</p>
		<p style="margin:0 0 16px;color:#1F1B2D;font-size:15px;line-height:1.6">Changed your mind? Sign in before the 30-day window closes and your account will be automatically restored.</p>
		%s
		<p style="margin:24px 0 0;color:#666;font-size:13px;line-height:1.6">If you did not request this deletion, sign in immediately to reactivate your account.</p>`,
		button(loginURL, "Sign in to reactivate"))
	return Message{
		To:      to,
		Subject: "Your Circl account has been scheduled for deletion",
		HTML:    wrap(frontendURL, "Account deletion scheduled", body),
		Text:    fmt.Sprintf("Your Circl account has been scheduled for deletion.\n\nYour account will be permanently deleted in 30 days. To cancel, sign in before the deadline:\n\n%s\n\nIf you did not request this, sign in immediately to reactivate your account.", loginURL),
	}
}

// AppealMessage returns the email sent when a user is suspended or banned.
// The appeal link routes to /appeal/{token} on the frontend, which the
// locked-out user can reach without logging in.
func AppealMessage(frontendURL, to, appealURL, reason string, permanent bool) Message {
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
		reasonBlock = fmt.Sprintf(`<p style="margin:0 0 16px;color:#666;font-size:13px;line-height:1.6"><strong>Reason given:</strong> %s</p>`, reason)
	}
	body := fmt.Sprintf(`
		<p style="margin:0 0 16px;color:#1F1B2D;font-size:15px;line-height:1.6">%s</p>
		%s
		<p style="margin:0 0 16px;color:#1F1B2D;font-size:15px;line-height:1.6">If you believe this is a mistake, you can submit an appeal. Our team reviews every appeal individually. This link expires in <strong>30 days</strong>.</p>
		%s`,
		intro, reasonBlock, button(appealURL, "Submit an appeal"))
	return Message{
		To:      to,
		Subject: subject,
		HTML:    wrap(frontendURL, headline, body),
		Text:    fmt.Sprintf("%s\n\n%s\n\nIf you believe this is a mistake, submit an appeal here (link expires in 30 days):\n\n%s\n\nQuestions: info.circl.ar@gmail.com", headline, intro, appealURL),
	}
}

// AppealResolutionMessage returns the email sent once admin approves or
// denies an appeal. Approved appeals reactivate the account; denied appeals
// leave the suspension in place.
func AppealResolutionMessage(frontendURL, to, loginURL, note string, approved bool) Message {
	headline := "Your appeal was approved"
	bodyText := "We have reviewed your appeal and reactivated your account. You can log in again now."
	cta := button(loginURL, "Sign in")
	if !approved {
		headline = "Your appeal was denied"
		bodyText = "We have reviewed your appeal and our decision stands."
		cta = ""
	}
	noteBlock := ""
	if note != "" {
		noteBlock = fmt.Sprintf(`<p style="margin:0 0 16px;color:#444;font-size:14px;line-height:1.6"><strong>Note from the review team:</strong> %s</p>`, note)
	}
	body := fmt.Sprintf(`
		<p style="margin:0 0 16px;color:#1F1B2D;font-size:15px;line-height:1.6">%s</p>
		%s
		%s`,
		bodyText, noteBlock, cta)
	return Message{
		To:      to,
		Subject: "Update on your Circl appeal",
		HTML:    wrap(frontendURL, headline, body),
		Text:    fmt.Sprintf("%s\n\n%s\n\n%s\n\nQuestions: info.circl.ar@gmail.com", headline, bodyText, note),
	}
}

// ExportReadyMessage returns the email sent when a user's data export is
// ready to download (Habeas Data / GDPR Art. 20). The download URL embeds a
// single-use 14-day token; the body restates the expiry so the user is not
// surprised when the link stops working.
func ExportReadyMessage(frontendURL, to, downloadURL string, expiresAt time.Time) Message {
	expires := expiresAt.UTC().Format("2006-01-02")
	body := fmt.Sprintf(`
		<p style="margin:0 0 16px;color:#1F1B2D;font-size:15px;line-height:1.6">The data export you requested is ready. The link below expires on <strong>%s</strong>.</p>
		%s
		<p style="margin:24px 0 0;color:#666;font-size:13px;line-height:1.6">The download is a single zip with a machine-readable JSON (<code>data.json</code>) and your media. If the link expires you can request a new export from Settings.</p>`,
		expires, button(downloadURL, "Download your data"))
	return Message{
		To:      to,
		Subject: "Your Circl data export is ready",
		HTML:    wrap(frontendURL, "Your data export is ready", body),
		Text:    fmt.Sprintf("Your Circl data export is ready.\n\nDownload here (expires %s):\n\n%s\n\nIf the link expires you can request a new export from Settings.\n\nQuestions: info.circl.ar@gmail.com", expires, downloadURL),
	}
}

// ExportFailedMessage returns the email sent when the data-export build fails
// so the user knows to retry rather than wait indefinitely.
func ExportFailedMessage(frontendURL, to string) Message {
	body := `
		<p style="margin:0 0 16px;color:#1F1B2D;font-size:15px;line-height:1.6">Something went wrong while building the data export you requested. No action was taken on your account — you can try again from Settings.</p>
		<p style="margin:24px 0 0;color:#666;font-size:13px;line-height:1.6">If this keeps happening, reply to this email or contact info.circl.ar@gmail.com.</p>`
	return Message{
		To:      to,
		Subject: "Your Circl data export could not be prepared",
		HTML:    wrap(frontendURL, "We couldn't prepare your data export", body),
		Text:    "We couldn't prepare your data export.\n\nNo action was taken on your account. Please try again from Settings; if this keeps happening, contact info.circl.ar@gmail.com.",
	}
}

// EmailVerificationMessage returns the email sent to verify a new account.
func EmailVerificationMessage(frontendURL, to, verifyURL string) Message {
	body := fmt.Sprintf(`
		<p style="margin:0 0 16px;color:#1F1B2D;font-size:15px;line-height:1.6">Click the button below to verify your email address and activate your account. This link expires in <strong>24 hours</strong>.</p>
		%s`, button(verifyURL, "Verify email"))
	return Message{
		To:      to,
		Subject: "Verify your Circl email address",
		HTML:    wrap(frontendURL, "Verify your email", body),
		Text:    fmt.Sprintf("Verify your Circl email address\n\n%s\n\nThis link expires in 24 hours.", verifyURL),
	}
}
