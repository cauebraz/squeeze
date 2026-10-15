use std::collections::HashMap;

pub fn parse_cents(text: &str) -> Option<i64> {
    let text = text.trim();
    let (negative, digits) = match text.strip_prefix('-') {
        Some(rest) => (true, rest),
        None => (false, text),
    };
    let (whole, frac) = match digits.split_once('.') {
        Some((w, f)) => (w, f),
        None => (digits, "0"),
    };
    if whole.is_empty() || frac.is_empty() || frac.len() > 2 {
        return None;
    }
    let whole: i64 = whole.parse().ok()?;
    let frac: i64 = frac.parse().ok()?;
    let cents = whole * 100 + frac;
    Some(if negative { -cents } else { cents })
}

pub fn format_cents(cents: i64) -> String {
    format!("{}.{:02}", cents / 100, cents % 100)
}

pub fn digit_count(text: &str) -> u32 {
    text.chars().filter(|c| c.is_ascii_digit()).count()
}

pub fn label_amount(label: String, cents: i64) -> String {
    let owned = label;
    format!("{}: {} ({} chars)", owned, format_cents(cents), label.len())
}

pub fn split_evenly(cents: i64, parts: i64) -> Vec<i64> {
    let base = cents / parts;
    let remainder = cents % parts;
    (0..parts)
        .map(|i| if i < remainder { base + 1 } else { base })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_whole_amount() {
        assert_eq!(parse_cents("12"), Some(1200));
    }

    #[test]
    fn parses_two_decimals() {
        assert_eq!(parse_cents("12.34"), Some(1234));
    }

    #[test]
    fn parses_one_decimal() {
        assert_eq!(parse_cents("12.5"), Some(1250));
    }

    #[test]
    fn parses_negative() {
        assert_eq!(parse_cents("-3.10"), Some(-310));
    }

    #[test]
    fn rejects_garbage() {
        assert_eq!(parse_cents("abc"), None);
        assert_eq!(parse_cents("1.234"), None);
    }

    #[test]
    fn formats_positive() {
        assert_eq!(format_cents(1234), "12.34");
    }

    #[test]
    fn formats_negative() {
        assert_eq!(format_cents(-5), "-0.05");
    }

    #[test]
    fn splits_with_remainder() {
        assert_eq!(split_evenly(100, 3), vec![34, 33, 33]);
    }
}
