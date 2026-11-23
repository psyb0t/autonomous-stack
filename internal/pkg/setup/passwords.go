package setup

import (
	"errors"
	"net/mail"
	"os"
	"path/filepath"

	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
)

const privateFileMode = 0o600

func prepareIdentity(cfg Config, root string) error {
	var email string

	if cfg.MailFrom != "" {
		address, err := mail.ParseAddress(cfg.MailFrom)
		if err != nil {
			return ctxerrors.Wrap(err, "parse agent mail identity")
		}

		email = address.Address
	}

	identity := map[string]any{
		"email":        email,
		"phone_number": cfg.PhoneNumber,
	}
	if cfg.local("mailbox") {
		identity["mailbox"] = map[string]string{
			"name":     "primary",
			"username": cfg.MailUsername,
			"password": cfg.MailPassword,
		}
	}

	if cfg.ModemBoxModemID != "" && cfg.modemConfigured() {
		identity["modembox_modem_id"] = cfg.ModemBoxModemID
	}

	if err := writeJSON(
		filepath.Join(root, "config/generated/identity.json"),
		identity,
	); err != nil {
		return ctxerrors.Wrap(err, "write agent identity")
	}

	if err := ensurePasswordFile(
		filepath.Join(root, "data/workspace/agent/private"),
	); err != nil {
		return ctxerrors.Wrap(err, "initialize account storage")
	}

	return nil
}

func ensurePasswordFile(directory string) error {
	if err := os.Mkdir(
		directory,
		privateDirectoryMode,
	); err != nil &&
		!errors.Is(err, os.ErrExist) {
		return ctxerrors.Wrap(err, "create private account directory")
	}

	info, err := os.Lstat(directory)
	if err != nil {
		return ctxerrors.Wrap(err, "inspect private account directory")
	}

	if !info.IsDir() || info.Mode().Perm() != privateDirectoryMode {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"account directory must be a real directory with mode 0700",
		)
	}

	path := filepath.Join(directory, "passwords.json")

	file, err := os.OpenFile(
		path,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		privateFileMode,
	)
	if errors.Is(err, os.ErrExist) {
		return validatePasswordFile(path)
	}

	if err != nil {
		return ctxerrors.Wrap(err, "create account password file")
	}

	_, writeErr := file.WriteString("{\"accounts\": []}\n")

	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return ctxerrors.Wrap(err, "initialize account password file")
	}

	return nil
}

func validatePasswordFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return ctxerrors.Wrap(err, "inspect existing account file")
	}

	if !info.Mode().IsRegular() || info.Mode().Perm() != privateFileMode {
		return ctxerrors.Wrap(
			commerr.ErrValidationFailed,
			"account file must be a regular file with mode 0600",
		)
	}

	return nil
}
