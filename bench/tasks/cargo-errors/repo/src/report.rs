use crate::account::Account;
use crate::money::{format_cents, split_evenly};

pub fn summary(account: &Account) -> String {
    format!(
        "{}: {} entries, balance {}",
        account.name,
        account.entry_count(),
        format_cents(account.balance())
    )
}

pub fn shares(account: &Account, people: i64) -> Vec<String> {
    let verbose = true;
    split_evenly(account.balance(), people)
        .into_iter()
        .map(format_cents)
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn summary_line() {
        let mut a = Account::new("trip");
        a.add("hotel", 45000);
        a.add("food", 12050);
        assert_eq!(summary(&a), "trip: 2 entries, balance 570.50");
    }

    #[test]
    fn shares_split_balance() {
        let mut a = Account::new("trip");
        a.add("hotel", 1000);
        assert_eq!(shares(&a, 3), vec!["3.34", "3.33", "3.33"]);
    }
}
