# Register page

Homework for novikontas.org

## Task

### 1. Name the one bottleneck you would fix first.

I would allow customer to register by themselves, without previously
   contacting support.

### 2. Explain why that one and not the others.

I've chosen this idea, as registration is the biggest bottleneck.
   Customer should be able to register without calling to support (e.g.,
   person is experienced seaman, and does not require any guidance to 
   book a refresher).

   My solution adds automatic payments (pay now option) and automatic
   insertion into Google Sheets. This saves at least 30 minutes of time
   for support team per customer, as well as drastically decreases the 
   number of misspellings done by employees.

   It is easy-medium problem to build and has low chance of breaking,
   since it does not exposes any write endpoint to the client - only to
   the payment system. 

   The other candidates would be:
   - Creating an web wrapper for internal CertBase, so that
     user/company is able to verify the certificate directly on the
     website, without sending any message to the support team. It lost
     because registrations are frequent, then certificate validity requests.
   - Implement personal account functionality for rescheduling, since 
     free rescheduling do not need the attention of support. It lost,
     because it's the next step of having registration page, that
     requires no support.

   On day one, I would ask specialists to update courses on Google
   Sheets, instead of sending updates by email, so that my future 
   automations would work correctly.

### 3. Build the fix.

The "fix" is in this repository.

#### 4. Show it working. 

https://github.com/xolra0d/novikontas-register-page/raw/refs/heads/main/example.mp4

### 5. Hand it over.

Register page homework has 3 main modules:
- Web server with list a list of courses and details to each. 
- *Stripe* integration for immediate checkout sessions (pay now option),
  and invoices (in test mode they do not send email -
  https://pkg.go.dev/github.com/stripe/stripe-go/v86/invoice#SendInvoice). 
- Google Sheets integration for automatic addition to the list(s).

Course list, which courses are available, courses information lives in
PostgreSQL. All other information remains in previous place. After
successful payment, server adds the user to the participants list in
Google Sheet. 

Server has a lot moved to compile time, instead of
run time, lot of info and error logs. It is not expected to receive
crash level errors, since input data from customer is not treated as
something more than a list of bytes, and all other input data is from
secure and trusted inputs - Stripe, Google. Logs are detailed so it's
easy to debug it and with small help of AI (if there is nobody who knows
how to code) it is possible to fix it.

### 6. What next.

I would know that it actually worked by checking and aggregating the
logs. 

If this would be a real idea, I would also add connection to the
scheduling mentioned in Homework, admin panel for easy updates in 
courses/schedule, atomic updates, seat registration (e.g., lock
on row in course participants, until either customer pays or 
2 minute timer runs out).

After this, I would build previously mentioned CertBase idea.

## AI usage

Copilot (later AI) was used as assistance, as it's free for students.

- AI was used to translate table of courses into insert. There is no
  need in explanation, since it's just standardizing table into SQL
  format.
- AI was used to find color scheme for front-end.
- AI was used to help find needed classes in bulma.io.

Back-end code, analysis and write-up was written by myself. Some of the
back-end code was taken from my previous projects.
