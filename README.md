# Firefly III Email Scanner

## About this project

This project provides a simple way to automatically insert transactions from an
email notification into [Firefly III](https://www.firefly-iii.org/).

To use it, you'll need a place to run the executable, preferably on a schedule
with cron or similar; a Firefly III instance; a Mattermost instance (optional,
for receiving notifications); an email to which you can get IMAP credentials;
and a bank which will send transaction alerts to that email.

The executable will read your recent, unread emails for transactions and then
check for close matches in your Firefly III instance. If one is not found, it
will create one based off of the information in the email. Either way, it will
then notify you via a message in a Mattermost channel (if configured) and mark
the email as read.

## Setup

The basic steps to getting up and running with the Firefly III Email Scanner
are:

1. Configure Firefly III
2. Configure a Mattermost bot (optional)
3. Set up an email account for notifications
4. Create email notifications
5. Create an env file (optional)
6. Create a configuration file
7. Install executable
8. Schedule executable (optional)

### Configuring Firefly III

To allow the email scanner to create transactions in Firefly, you'll need to
[create a personal access token (PAT)](https://docs.firefly-iii.org/how-to/firefly-iii/features/api/#personal-access-tokens).
Note that this will expire eventually and you'll have to repeat this process to
get a new one.

For future steps, take note of your Firefly III URL and your PAT.

### Configure a Mattermost bot (Optional)

You may optionally configure a Mattermost instance and bot account for
notifications about actions taken by the email scanner.

To set up a Mattermost bot and add it to your channel, follow
[these directions](https://developers.mattermost.com/integrate/reference/bot-accounts/).
I created a new channel specifically for these notifications.

For future steps, find your channel ID and your bot PAT.

### Setting up the email

I recommend using an email dedicated solely to your email notifications, if
possible. This makes it less likely that a bug in this project does something
you regret to your email. I created a new Gmail account for mine.

Regardless of the email you use, you'll need a way for this application to
authenticate via
[IMAP](https://en.wikipedia.org/wiki/Internet_Message_Access_Protocol). To do
this with Gmail, you'll need to configure
[an app password](https://support.google.com/mail/answer/185833?hl=en). Note
that this is not recommended in most scenarios and is another good reason to use
a separate email if you go the Gmail route.

For future steps, take note of the IMAP server address and port, your
email/username and the password.

### Setting up email notifications

It is not possible to provide and maintain detailed instructions for this step
because of the variance among banks and the continual changes they all make. You
will need to go poke around the settings for your banking portal - probably in
some notifications, alerts or messaging section - and find the place to
configure this, if it exists.

If you have decided to use a dedicated email for these notifications but your
bank does not allow you to register multiple notification emails, consider
keeping your original email on file with them but creating a forwarding rule in
your original email to forward these notifications to your dedicated email.

### Create an env file

The email scanner uses environment variables to retrieve most of its secrets
values. I have found using an env file to be the most straight-forward and
repeatable option, but you can do whatever makes sense in your deployment
scenario. The following is a template for an env file containing the required
secrets:

```env
IMAP_SERVER="imap.gmail.com:993"
IMAP_EMAIL="your-cool-dedicated-email-address@gmail.com"
IMAP_PASSWORD="some value here"
FIREFLY_URL="https://your.firefly.domain.com"
FIREFLY_PAT="your firefly PAT"
MATTERMOST_URL="https://mm.exultantsoftware.com"
MATTERMOST_CHANNEL="mattermost channel id"
MATTERMOST_TOKEN="mattermost bot token"
```

### Create a configuration file

The configuration file informs the program what emails to look for and how to
process them. The config file format is a work in progress, but the current
syntax is explained here:

```yaml
# Configuration for the mattermost notifier. Optional.
# You may also set `notifier: stdout` if you would like the notifications
# printed to stdout for debugging, instead.
notifier: mattermost
# The root list of processing steps, required.
# Each object in the list contains a instructions per bank "from" email
process_emails:
  # The email that the bank uses to send the emails to you
  - fromEmail: alerts@mybank.com
    # A priority order list of the potential types of emails, probably one per account.
    processingSteps:
      # Friendly name, just for you.
      - optionName: MyBank Money Market
        # The ID of the asset account the email will be about.
        # You can find this in the URL while viewing the asset account in Firefly
        sourceAccountId: 3
        # A rule for how to tell if an email "matches." In other worsd, for the given from email, if this regex matches, this is the rule to use.
        discriminator:
          # The type of rule. This is the only currently implemented type.
          type: plainTextBodyRegex
          # A regex to find in the body of the email.
          # Make sure it is something that is _uniquely_ in this email type (e.g. last 4 of the account number, the text "new transaction", etc)
          regex: "Online Money Market account"
        # A list of steps to run to extract relevant information from the email.
        # Each step is a regex with capture groups that map to target fields (one of dollars, cents, transactionDate, destinationAccount)
        extractionSteps:
          # The type of step. Again, only plainTextBodyRegex is implemented.
          - type: plainTextBodyRegex
            # The regex to search for and to extract values from.
            regex: "for \\$([\\d,]+)\\.(\\d{2}) is above"
            targetFields:
              - groupNumber: 1 # The group in the regex
                targetField: dollars # The field to use (dollars)
              - groupNumber: 2
                targetField: cents
          - type: plainTextBodyRegex
            regex: "A transaction on ([^ ]+)"
            targetFields:
              - groupNumber: 1
                targetField: transactionDate
                timeZone: "America/Denver" # For the transactionDate, you *must* provide a IANA Time Zone database formatted name
                format: "01/02/2006" # For the transactionDate, you *can* provide a Go date format string for parsing.
          - type: plainTextBodyRegex
            regex: "^To:(.+)$"
            targetFields:
              - groupNumber: 1
                targetField: destinationAccount # This is a string and will be fuzzy matched against existing expense accounts for a best guess.
```

### Install executable

The executable can be built from source or downloaded from
[the GitHub releases on this project](https://github.com/kennethac/firefly-iii-email-scanner/releases).

### Schedule Executable

If you don't want to run the executable manually, you can set up a cron job or
similar to run it on a schedule. For example, I have the following in my
crontab:

```
*/15 * * * * /path-to-my-bin/run.sh
```

And the `run.sh` file takes care of supplying the environment variables:

```bash
#!/usr/bin/env bash
cd /path-to-my-bin
set -o allexport
source gmail.env
./firefly-iii-email-scanner  >> log.txt 2>&1
```
