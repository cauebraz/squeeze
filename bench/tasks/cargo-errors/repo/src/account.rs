#[derive(Debug, Clone, PartialEq)]
pub struct Entry {
    pub label: String,
    pub amount: i64,
}

#[derive(Debug, Default)]
pub struct Account {
    pub name: String,
    pub entries: Vec<Entry>,
    pub log: Vec<String>,
}

impl Account {
    pub fn new(name: &str) -> Account {
        Account {
            name: name.to_string(),
            entries: Vec::new(),
            log: Vec::new(),
        }
    }

    pub fn add(&mut self, label: &str, amount: i64) {
        self.entries.push(Entry {
            label: label.to_string(),
            amount,
        });
    }

    pub fn balance(&self) -> i64 {
        self.entries.iter().map(|e| e.amount).sum()
    }

    pub fn entry_count(&self) -> u32 {
        self.entries.len() as u32
    }

    fn largest_mut(&mut self) -> Option<&mut Entry> {
        self.entries.iter_mut().max_by_key(|e| e.amount)
    }

    pub fn rename_largest(&mut self, label: &str) {
        self.log.push(format!("rename largest to {}", label));
        let largest = self.largest_mut();
        if let Some(entry) = largest {
            entry.label = label.to_string();
        }
    }

    pub fn apply_fee(&mut self, fee: i64) {
        let threshold = 0;
        if self.balance() < fee {
            self.log.push(format!("fee {} skipped", fee));
            return;
        }
        self.add("fee", -fee);
    }
}

fn audit_trail(account: &Account) -> String {
    account.log.join("\n")
}

#[cfg(test)]
mod tests {
    use super::*;

    fn sample() -> Account {
        let mut a = Account::new("main");
        a.add("salary", 300000);
        a.add("rent", -120000);
        a.add("bonus", 50000);
        a
    }

    #[test]
    fn balance_sums_entries() {
        assert_eq!(sample().balance(), 230000);
    }

    #[test]
    fn counts_entries() {
        assert_eq!(sample().entry_count(), 3);
    }

    #[test]
    fn renames_largest_entry() {
        let mut a = sample();
        a.rename_largest("paycheck");
        assert_eq!(a.entries[0].label, "paycheck");
        assert_eq!(a.log.len(), 1);
    }

    #[test]
    fn fee_is_charged() {
        let mut a = sample();
        a.apply_fee(500);
        assert_eq!(a.balance(), 229500);
    }

    #[test]
    fn fee_skipped_when_short() {
        let mut a = Account::new("empty");
        a.apply_fee(500);
        assert_eq!(a.balance(), 0);
        assert_eq!(a.log, vec!["fee 500 skipped".to_string()]);
    }
}
