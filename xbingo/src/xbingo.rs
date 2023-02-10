#![no_std]

elrond_wasm::imports!();
elrond_wasm::derive_imports!();

#[derive(TopEncode, TopDecode, TypeAbi, PartialEq, Clone, Copy)]
pub enum Status {
    Running,
    Extracting,
    Idle,
    Paused
}

#[derive(NestedEncode, NestedDecode, TopEncode, TopDecode, TypeAbi)]
pub struct TicketInfo<M: ManagedTypeApi> {
    pub lines: ArrayVec::<BigUint<M>, 3>,
}

const MAX_NUMBERS: usize = 90;
const NUMBERS_PER_TICKET: usize = 15;
const TICKET_COLUMNS: usize = 9;
const TICKET_ROWS: usize = 3;
const DEFAULT_ROUND_DURATION: u64 = 50_u64; // 5 minutes
const DEFAULT_TICKET_PRICE: u64 = 100_000_000_000_000_000u64;
const DEFAULT_NUMBERS_TO_EXTRACT: usize = 65;
const DEFAULT_BINGO_PRIZE_MULTIPLIER: u64 = 20;
const DEFAULT_ONE_LINE_PRIZE_MULTIPLIER: u64 = 1;
const DEFAULT_TWO_LINES_PRIZE_MULTIPLIER: u64 = 2;

#[derive(Default)]
pub struct Matrix {
    rows: [[usize; TICKET_COLUMNS]; TICKET_ROWS],
}

impl Matrix {
    pub fn get(&self, row: usize, col: usize) -> Option<usize> {
        let val = self.rows.get(row)?.get(col)?;
        Some(*val)
    }
}

#[elrond_wasm::contract]
pub trait Bingo {
 
    #[view(getRound)]
    #[storage_mapper("game_round")]
    fn game_round(&self) -> SingleValueMapper<u64>;

    #[view(getNumbersToExtract)]
    #[storage_mapper("numbers_to_extract")]
    fn numbers_to_extract(&self) -> SingleValueMapper<usize>;

    #[view(getLastExtractedNumbers)]
    #[storage_mapper("last_extracted_numbers")]
    fn last_extracted_numbers(&self) -> SingleValueMapper<BigUint>;

    #[view(getTokenIdentifier)]
    #[storage_mapper("token_identifier")]
    fn token_identifier(&self) -> SingleValueMapper<EgldOrEsdtTokenIdentifier>;

    #[view(getTicketPrice)]
    #[storage_mapper("ticket_price")]
    fn ticket_price(&self) -> SingleValueMapper<BigUint>;

    #[view(getBingoPrizeMultiplier)]
    #[storage_mapper("bingo_prize_multiplier")]
    fn bingo_prize_multiplier(&self) -> SingleValueMapper<u64>;

    #[view(getOneLinePrizeMultiplier)]
    #[storage_mapper("one_line_prize_multiplier")]
    fn one_line_prize_multiplier(&self) -> SingleValueMapper<u64>;

    #[view(getTwoLinesPrizeMultiplier)]
    #[storage_mapper("two_lines_prize_multiplier")]
    fn two_lines_prize_multiplier(&self) -> SingleValueMapper<u64>;

    #[view(getDeadline)]
    #[storage_mapper("deadline")]
    fn deadline(&self) -> SingleValueMapper<u64>;

    #[view(getPlayers)]
    #[storage_mapper("player")]
    fn players(&self) -> UnorderedSetMapper<ManagedAddress>;

    #[view(getRoundDuration)]
    #[storage_mapper("round_duration")]
    fn round_duration(&self) -> SingleValueMapper<u64>;

    #[view(getUserTickets)]
    #[storage_mapper("ticket")]
    fn tickets(&self, player: &ManagedAddress) -> VecMapper<TicketInfo<Self::Api>>;
  
    #[view(getRoundTickets)]
    #[storage_mapper("round_ticket")]
    fn round_tickets(&self) -> SingleValueMapper<u64>;
  
    #[view(getAllTimeTickets)]
    #[storage_mapper("total_tickets")]
    fn total_tickets(&self) -> SingleValueMapper<u64>;
  
    #[view(getAllTimeBingo)]
    #[storage_mapper("total_bingo")]
    fn total_bingo(&self) -> SingleValueMapper<u64>;

    #[view(getAllTimeTwoLines)]
    #[storage_mapper("total_two_lines")]
    fn total_two_lines(&self) -> SingleValueMapper<u64>;

    #[view(getAllTimeOneLine)]
    #[storage_mapper("total_one_line")]
    fn total_one_line(&self) -> SingleValueMapper<u64>;
  
    #[storage_mapper("paused")]
    fn paused(&self) -> SingleValueMapper<usize>;

    #[view(getStatus)]
    fn status(&self) -> Status {
        if self.paused().get() == 1 {
            Status::Paused
        } else if self.blockchain().get_block_round() < self.deadline().get() {
            Status::Running
        } else if self.players().len() != 0 {
            Status::Extracting
        } else {
            Status::Idle
        }
    }



    #[init]
    fn init(&self) {
        self.ticket_price().set(BigUint::from(DEFAULT_TICKET_PRICE));
        let current_time = self.blockchain().get_block_round();
        self.deadline().set(current_time);
        self.round_duration().set(DEFAULT_ROUND_DURATION);
        self.numbers_to_extract().set(DEFAULT_NUMBERS_TO_EXTRACT);
        self.bingo_prize_multiplier().set(DEFAULT_BINGO_PRIZE_MULTIPLIER);
        self.one_line_prize_multiplier().set(DEFAULT_ONE_LINE_PRIZE_MULTIPLIER);
        self.two_lines_prize_multiplier().set(DEFAULT_TWO_LINES_PRIZE_MULTIPLIER);
        self.game_round().set(0);
        self.total_tickets().set(0);
        self.total_bingo().set(0);
        self.total_two_lines().set(0);
        self.total_one_line().set(0);
        self.paused().set(0);
    }



    #[endpoint]
    #[payable("*")]
    fn fund(
        &self,
    ) -> SCResult<()> {
        Ok(())
    }

    #[endpoint]
    #[payable("*")]
    fn buy_ticket(
        &self,
        // #[payment_amount] payment: BigUint,
    ) -> SCResult<ArrayVec::<BigUint, TICKET_ROWS>> {
        let (token_id, payment) = self.call_value().egld_or_single_fungible_esdt();

        require!(self.status() != Status::Paused, "game paused");
        require!(self.status() != Status::Extracting, "extracting numbers");
        require!(payment == self.ticket_price().get(), "wrong amount. check ticket price");
        require!(token_id == self.token_identifier().get(), "wrong payment coin");

        if self.status() == Status::Idle {
            self.start_game();
        }

        let caller = self.blockchain().get_caller();
        let mut biguintticket = BigUint::from(0_u64);
        let mut numbers_generated = 0;
        let mut rand = RandomnessSource::<Self::Api>::new();
        let mut numbers_per_columns = ArrayVec::<usize, TICKET_COLUMNS>::new();
        let mut numbers_per_rows = ArrayVec::<usize, TICKET_ROWS>::new();
        let mut rawticket: Matrix = Default::default();
        let mut ticket: Matrix = Default::default();

        // generate ticket numbers
        for column in 0..TICKET_COLUMNS {
            // generate number of numbers per column
            let mut from = 1;
            let mut to = 3;
            let left_columns = TICKET_COLUMNS - column;
            let left_numbers = NUMBERS_PER_TICKET - numbers_generated;
        
            if numbers_generated + left_columns > NUMBERS_PER_TICKET - TICKET_ROWS + 1 {
                to = left_numbers - (left_columns - 1);
            }
        
            if numbers_generated + left_columns * TICKET_ROWS < NUMBERS_PER_TICKET + TICKET_ROWS - 1 {
                from = left_numbers - (left_columns - 1) * TICKET_ROWS;
            }
        
            let numbers_per_column = rand.next_usize_in_range(from, to + 1);
            numbers_generated += numbers_per_column;
          
            // generate random numbers in column
            let rnumbers = self.get_distinct_random(1, 10, numbers_per_column);
            let mut numbers = ArrayVec::<usize, TICKET_ROWS>::new();
            for i in 0..numbers_per_column {
                numbers.push(rnumbers[i])
            }
            numbers.sort();

            for i in 0..numbers_per_column {
                numbers[i] += column * 10;
                let mut m = BigUint::from(2_u64);
                m = m.pow(numbers[i] as u32);
                biguintticket = biguintticket.add(m);
                rawticket.rows[i][column] = numbers[i];
            }

            numbers_per_columns.push(numbers_per_column);
        }

        // assign numbers on rows
        for _row in 0..TICKET_ROWS {
            numbers_per_rows.push(0);
        }

        let mut ticket_lines = TicketInfo {
            lines: ArrayVec::<BigUint, 3>::new(),
        };

        for row in 0..TICKET_ROWS {
            numbers_per_rows[row] = 0;
            let mut biguint = BigUint::from(0_u64);
            for column in 0..TICKET_COLUMNS {
                if numbers_per_columns[column] == TICKET_ROWS - row {
                    let number = rawticket.rows[0][column];
                    let mut m = BigUint::from(2_u64);
                    m = m.pow(number as u32);
                    biguint = biguint.add(m);
                    ticket.rows[row][column] = number;
                    rawticket.rows[0][column] = rawticket.rows[1][column];
                    rawticket.rows[1][column] = rawticket.rows[2][column];
                    numbers_per_columns[column] -= 1;
                    numbers_per_rows[row] += 1;
                }
            }
            let rnd_columns = self.get_distinct_random(0, TICKET_COLUMNS - 1, TICKET_COLUMNS);
            for column in rnd_columns { // randomize order here
                if (ticket.rows[row][column] == 0) && (numbers_per_columns[column] > 0) {
                    let number = rawticket.rows[0][column];
                    let mut m = BigUint::from(2_u64);
                    m = m.pow(number as u32);
                    biguint = biguint.add(m);
                    ticket.rows[row][column] = number;
                    rawticket.rows[0][column] = rawticket.rows[1][column];
                    rawticket.rows[1][column] = rawticket.rows[2][column];
                    numbers_per_columns[column] -= 1;
                    numbers_per_rows[row] += 1;
                    if numbers_per_rows[row] == 5 {
                        break;
                    }
                }
            }
            ticket_lines.lines.push(biguint);
        }
        
        let mut rt = self.round_tickets().get();
        rt += 1;
        self.round_tickets().set(&rt);

        let mut tt = self.total_tickets().get();
        tt += 1;
        self.total_tickets().set(&tt);

        self.tickets(&caller).push(&ticket_lines);
        self.players().insert(caller);

        Ok(ticket_lines.lines)
    }

    #[only_owner]
    #[endpoint]
    fn start(&self) -> SCResult<()> {
        require!(self.status() == Status::Idle, "game not finished or paused");
        self.start_game();
        Ok(())
    }
    
    fn start_game(&self) {
        self.generate_winning_numbers();
        let current_time = self.blockchain().get_block_round();
	let duration = self.round_duration().get();
        self.deadline().set(current_time + duration);
        let mut round = self.game_round().get();
        round += 1;
        self.game_round().set(&round);
        self.round_tickets().set(0);
    }

    #[endpoint]
    fn extract_numbers(&self) -> SCResult<()> {
        require!(self.status() != Status::Paused, "game paused");
        let current_time = self.blockchain().get_block_round();
        require!(current_time > self.deadline().get(), "game still running");
        require!(self.status() == Status::Extracting, "game not started");

        let winning_numbers = self.last_extracted_numbers().get();
        let token_id = self.token_identifier().get();

        for player in self.players().iter() {
            for ticket in self.tickets(&player).iter() {
                let mut lines = 0;
                for line in ticket.lines.iter() {
                    if (line & &winning_numbers).eq(line) {
                        lines += 1;
                    }
                }
                let price = self.ticket_price().get();
                if lines == 3 {
                    let prize = price * self.bingo_prize_multiplier().get();
                    self.send().direct(&player, &token_id, 0, &prize, &[]);
                    // self.send().direct_egld(&player, &prize, b"Bingo!");
                    let mut bingo = self.total_bingo().get();
                    bingo += 1;
                    self.total_bingo().set(&bingo);
                } else if lines == 2 {
                    let prize = price * self.two_lines_prize_multiplier().get();
                    self.send().direct(&player, &token_id, 0, &prize, &[]);
                    // self.send().direct_egld(&player, &prize, b"2 Lines!");
                    let mut two_lines = self.total_two_lines().get();
                    two_lines += 1;
                    self.total_two_lines().set(&two_lines);
                } else if lines == 1 {
                    let prize = price * self.one_line_prize_multiplier().get();
                    self.send().direct(&player, &token_id, 0, &prize, &[]);
                    // self.send().direct_egld(&player, &prize, b"Line!");
                    let mut one_line = self.total_one_line().get();
                    one_line += 1;
                    self.total_one_line().set(&one_line);
                }
            }
            self.tickets(&player).clear();
        }
        self.players().clear();
        
        Ok(())
    }

    fn generate_winning_numbers(&self) {
        // generate winning numbers
        let numbers_to_extract = self.numbers_to_extract().get();
        let extracted_numbers = self.get_distinct_random(1, MAX_NUMBERS, numbers_to_extract);
        let mut random = BigUint::from(0_u64);
        
        for i in 0..numbers_to_extract {
            let mut n = BigUint::from(2_u64);
            n = n.pow(extracted_numbers[i] as u32);
            random = random.add(n);
        }

        self.last_extracted_numbers().set(&random);
    }

    fn get_distinct_random(
        &self,
        min: usize,
        max: usize,
        amount: usize,
    ) -> ArrayVec<usize, MAX_NUMBERS> {
        let mut rand_numbers = ArrayVec::new();

        for num in min..=max {
            rand_numbers.push(num);
        }

        let total_numbers = rand_numbers.len();
        let mut rand = RandomnessSource::<Self::Api>::new();

        for i in 0..amount {
            let rand_index = rand.next_usize_in_range(0, total_numbers);
            rand_numbers.swap(i, rand_index);
        }

        rand_numbers
    }

    #[only_owner]
    #[endpoint]
    fn set_numbers_to_extract(&self, numbers: usize) -> SCResult<()> {
        require!(self.status() == Status::Idle, "game must be idle");
        require!(numbers < MAX_NUMBERS, "max is 90");
        require!(numbers >= 45, "min is 45");

        self.numbers_to_extract().set(&numbers);

        Ok(())
    }

    #[only_owner]
    #[endpoint]
    fn set_prize_multipliers(&self, one_line: u64, two_lines: u64, bingo: u64) -> SCResult<()> {
        self.bingo_prize_multiplier().set(&bingo);
        self.one_line_prize_multiplier().set(&one_line);
        self.two_lines_prize_multiplier().set(&two_lines);

        Ok(())
    }

    #[only_owner]
    #[endpoint]
    fn set_ticket_price(&self, new_price: BigUint) -> SCResult<()> {
        self.ticket_price().set(new_price);

        Ok(())
    }

    #[only_owner]
    #[endpoint]
    fn set_round_duration(&self, new_duration: u64) -> SCResult<()> {
        self.round_duration().set(&new_duration);

        Ok(())
    }

    #[only_owner]
    #[endpoint]
    fn claim_winnings(&self) -> SCResult<()> {
        require!(self.status() == Status::Idle, "game must be idle");

        let token_id = self.token_identifier().get();

        let balance = self.blockchain().get_sc_balance(&token_id, 0);
        let caller = self.blockchain().get_caller();
        self.send().direct(&caller, &token_id, 0, &balance, &[]);

        Ok(())
    }

    #[only_owner]
    #[endpoint]
    fn pause(&self) -> SCResult<()> {
        self.paused().set(1);
        
        Ok(())
    }

    #[only_owner]
    #[endpoint]
    fn resume(&self) -> SCResult<()> {
        self.paused().set(0);
        
        Ok(())
    }

    #[only_owner]
    #[endpoint]
    fn set_token_identifier(&self, new_token_id: EgldOrEsdtTokenIdentifier) -> SCResult<()> {
        self.token_identifier().set(new_token_id);

        Ok(())
    }
}
