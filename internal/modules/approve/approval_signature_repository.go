package approve

import (
	"context"
	"database/sql"
	"errors"
)

func (r *ActionRepository) SaveSignature(ctx context.Context, userID, image string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_signatures (user_id, image, updated_at) VALUES ($1, $2, now())
		ON CONFLICT (user_id) DO UPDATE SET image = EXCLUDED.image, updated_at = now()`,
		userID, image)
	return err
}

func (r *ActionRepository) GetSignature(ctx context.Context, userID string) (string, error) {
	var img string
	err := r.db.QueryRowContext(ctx,
		`SELECT image FROM user_signatures WHERE user_id = $1`, userID).Scan(&img)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return img, nil
}

func (r *ActionRepository) DeleteSignature(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_signatures WHERE user_id = $1`, userID)
	return err
}

func (r *ActionRepository) CopySignatureToStep(ctx context.Context, tx *sql.Tx, userID, stepID string) error {
	res, err := tx.ExecContext(ctx, `
	UPDATE approval_steps SET signature_image = s.image
	FROM user_signatures s
	WHERE s.user_id = $1 AND approval_steps.id = $2`, userID, stepID)

	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoSignature
	}
	return nil
}

func (r *ActionRepository) SignaturesFor(ctx context.Context, requestID string) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT code, signature_image FROM approval_steps
		WHERE request_id = $1
			AND round = (SELECT approval_round FROM requests WHERE id = $1)
			AND signature_image IS NOT NULL
		UNION ALL
		SELECT 'requester', requester_sig_image FROM requests
		WHERE id = $1 AND requester_sig_image IS NOT NULL
		UNION ALL
		SELECT 'date:'|| code, to_char(acted_at AT TIME ZONE 'Asia/Makassar', 'DD/MM/YYYY') FROM approval_steps
		WHERE request_id = $1
			AND round = (SELECT approval_round FROM requests WHERE id = $1)
			AND status = 'approved' AND acted_at IS NOT NULL
		UNION ALL
		SELECT 'date:requester', to_char(created_at AT TIME ZONE 'Asia/Makassar', 'DD/MM/YYYY') FROM requests
		WHERE id = $1`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var code, img string
		if err := rows.Scan(&code, &img); err != nil {
			return nil, err
		}
		out[code] = img
	}
	return out, rows.Err()
}

func (r *ActionRepository) AttachRequesterSignature(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE requests q SET requester_sig_image = s.image
		FROM user_signatures s
		WHERE s.user_id = $1
		  AND q.created_by_user_id::text = $1
		  AND q.requester_sig_image IS NULL`, userID)
	return err
}

func (r *ActionRepository) AttachSignatures(ctx context.Context, requestID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE approval_steps st
		SET signature_image = s.image
		FROM user_signatures s
		WHERE st.request_id = $1
		  AND st.acted_by_user_id IS NOT NULL
		  AND st.signature_image IS NULL
		  AND s.user_id = st.acted_by_user_id`, requestID)
	return err
}
