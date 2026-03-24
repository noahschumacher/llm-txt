# Memory

## feedback_comments.md
- **Name:** concise comments
- **Description:** User wants code comments to be short and to the point, not verbose paragraphs
- **Type:** feedback
- **Summary:** Keep comments to one line (two at most). Explain the non-obvious "why", not what the code already says. Skip comments that restate obvious behavior.

## feedback_nolint.md
- **Name:** no nolint comments
- **Description:** User does not want //nolint comments added to code
- **Type:** feedback
- **Summary:** Do not add //nolint:errcheck or any other //nolint directives. If an error is intentionally ignored, leave it without annotation or handle it properly.

## feedback_exports.md
- **Name:** unexported by default
- **Description:** Only export identifiers that must be public; default to unexported
- **Type:** feedback
- **Summary:** Default to unexported types, functions, and fields. Only capitalize if the identifier needs to be used outside the package.

## feedback_log_or_return.md
- **Name:** log or return errors, not both
- **Description:** Only log an error when it is not being returned up the call stack
- **Type:** feedback
- **Summary:** Never both log an error and return it. In service/helper functions, return errors unwrapped. Only log at the boundary where the error is handled and not returned further (e.g. HTTP handlers).

## feedback_comment_wrap.md
- **Name:** comment line width
- **Description:** Wrap comments at 80 chars; inline field comments on same line may exceed 80
- **Type:** feedback
- **Summary:** Block comments wrap at 80 characters. Inline comments on the same line as a field or statement are exempt from the 80-char limit.

## feedback_slices.md
- **Name:** slice and map initialization
- **Description:** Always use make() for slices and maps, with size hints when known
- **Type:** feedback
- **Summary:** Use make([]T, 0) not []T{}, make(map[K]V) not map[K]V{}. Pass size hint when known. Maps are especially important — rehashing on growth is expensive.
