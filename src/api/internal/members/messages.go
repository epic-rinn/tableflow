package members

import "github.com/epic-rinn/tableflow/src/api/internal/platform/mail"

// Plain-text emails in the member's locale. Links carry the token in the URL
// fragment so it is never sent to a server or in a Referer header.

func verifyMessage(to, locale, base, token string) mail.Message {
	link := base + "/account/verify#" + token
	if locale == "th" {
		return mail.Message{To: to, Kind: "member.verify", Subject: "ยืนยันอีเมล TableFlow",
			Text: "กรุณายืนยันอีเมลของคุณภายใน 24 ชั่วโมง:\n" + link + "\n\nหากคุณไม่ได้สมัครสมาชิก ไม่ต้องดำเนินการใด ๆ\n"}
	}
	return mail.Message{To: to, Kind: "member.verify", Subject: "Confirm your TableFlow email",
		Text: "Confirm your email within 24 hours:\n" + link + "\n\nIf you did not sign up, you can ignore this message.\n"}
}

func existingAccountMessage(to, locale, base string) mail.Message {
	link := base + "/account/reset"
	if locale == "th" {
		return mail.Message{To: to, Kind: "member.exists", Subject: "บัญชี TableFlow ของคุณ",
			Text: "มีการสมัครด้วยอีเมลนี้ ซึ่งมีบัญชีอยู่แล้ว หากลืมรหัสผ่าน ตั้งใหม่ได้ที่:\n" + link + "\n"}
	}
	return mail.Message{To: to, Kind: "member.exists", Subject: "Your TableFlow account",
		Text: "Someone tried to sign up with this email, which already has an account. If you forgot your password, reset it here:\n" + link + "\n"}
}

func resetMessage(to, locale, base, token string) mail.Message {
	link := base + "/account/reset/confirm#" + token
	if locale == "th" {
		return mail.Message{To: to, Kind: "member.reset", Subject: "ตั้งรหัสผ่าน TableFlow ใหม่",
			Text: "ตั้งรหัสผ่านใหม่ภายใน 1 ชั่วโมง:\n" + link + "\n\nหากคุณไม่ได้ร้องขอ ไม่ต้องดำเนินการใด ๆ\n"}
	}
	return mail.Message{To: to, Kind: "member.reset", Subject: "Reset your TableFlow password",
		Text: "Set a new password within 1 hour:\n" + link + "\n\nIf you did not ask for this, you can ignore this message.\n"}
}
