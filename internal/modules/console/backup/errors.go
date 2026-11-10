package backup

import apperrors "github.com/cng1985/dbgo/internal/platform/errors"

func invalid(message string) error { return apperrors.Invalid(message) }

func missing(message string) error { return apperrors.NotFound(message) }
