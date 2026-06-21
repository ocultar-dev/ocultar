import json
import os

# We will define samples as: (text, [(substring, entity_type), ...], category)
# The script will automatically locate start and end positions of each substring in the text
# and verify that it matches exactly. This avoids offset errors.

raw_samples = [
    # ── Category 1: Basic PII (EN, FR, ES, DE) ──
    (
        "Alice Smith living at 123 Main St, New York, NY 10001, phone 555-0199, email alice.smith@outlook.com, SSN 000-12-3456, passport USA123456.",
        [
            ("Alice Smith", "PERSON"),
            ("123 Main St, New York, NY 10001", "ADDRESS"),
            ("555-0199", "PHONE"),
            ("alice.smith@outlook.com", "EMAIL"),
            ("000-12-3456", "SSN"),
            ("USA123456", "PASSPORT")
        ],
        "basic_pii"
    ),
    (
        "Jean Dupont habite au 45 Rue de la Paix, 75002 Paris, France, téléphone +33 1 42 27 78 90, email jean.dupont@gmail.fr, numéro de sécurité sociale 180027512345678, passeport FR987654.",
        [
            ("Jean Dupont", "PERSON"),
            ("45 Rue de la Paix, 75002 Paris, France", "ADDRESS"),
            ("+33 1 42 27 78 90", "PHONE"),
            ("jean.dupont@gmail.fr", "EMAIL"),
            ("180027512345678", "SSN"),
            ("FR987654", "PASSPORT")
        ],
        "basic_pii"
    ),
    (
        "María García, calle Mayor 12, Madrid 28013, España, móvil +34 600 123 456, email maria.garcia@yahoo.es, DNI 12345678A, pasaporte ES112233.",
        [
            ("María García", "PERSON"),
            ("calle Mayor 12, Madrid 28013, España", "ADDRESS"),
            ("+34 600 123 456", "PHONE"),
            ("maria.garcia@yahoo.es", "EMAIL"),
            ("12345678A", "SSN"), # National ID maps to SSN in general PII config
            ("ES112233", "PASSPORT")
        ],
        "basic_pii"
    ),
    (
        "Hans Müller, Hauptstraße 10, 10117 Berlin, Deutschland, Tel +49 30 1234567, E-Mail hans.mueller@web.de, Steuernummer 112/345/67890, Personalausweis DE5544332.",
        [
            ("Hans Müller", "PERSON"),
            ("Hauptstraße 10, 10117 Berlin, Deutschland", "ADDRESS"),
            ("+49 30 1234567", "PHONE"),
            ("hans.mueller@web.de", "EMAIL"),
            ("112/345/67890", "TAX_ID"),
            ("DE5544332", "NATIONAL_ID")
        ],
        "basic_pii"
    ),

    # ── Category 2: Enterprise Data ──
    (
        "Hi Support team, this is Robert Johnson. I can't access my dashboard. My billing address is 789 Pine Rd, Chicago, IL. You can email me at rjohnson@enterprise.com or call +1 312 555 0143.",
        [
            ("Robert Johnson", "PERSON"),
            ("789 Pine Rd, Chicago, IL", "ADDRESS"),
            ("rjohnson@enterprise.com", "EMAIL"),
            ("+1 312 555 0143", "PHONE")
        ],
        "enterprise_data"
    ),
    (
        "Hey Team, we got a new enterprise lead from Acme Corp. Contact person is Sarah Connor (sconnor@acme.com, +1 415 555 9876). She is interested in our API security pipeline.",
        [
            ("Sarah Connor", "PERSON"),
            ("sconnor@acme.com", "EMAIL"),
            ("+1 415 555 9876", "PHONE")
        ],
        "enterprise_data"
    ),
    (
        "CRM Export entry: ID 9928, Customer David Lee, Company TechCorp, Email dlee@techcorp.io, Phone +1 650 555 1212, Address 101 Silicon Valley Blvd, San Jose, CA.",
        [
            ("David Lee", "PERSON"),
            ("dlee@techcorp.io", "EMAIL"),
            ("+1 650 555 1212", "PHONE"),
            ("101 Silicon Valley Blvd, San Jose, CA", "ADDRESS")
        ],
        "enterprise_data"
    ),

    # ── Category 3: Medical Data ──
    (
        "Patient John Doe (DOB 1980-05-12) presented with acute symptoms. MRN: MRN-9988776. Insurance ID: BLUE-900811. Discharged to 44 Health Lane, Boston.",
        [
            ("John Doe", "PERSON"),
            ("MRN-9988776", "MEDICAL_RECORD"),
            ("BLUE-900811", "PATIENT_ID"),
            ("44 Health Lane, Boston", "ADDRESS")
        ],
        "medical_data"
    ),
    (
        "Clinical note for patient Emily Davis, patient ID MED-8877112. Contact number is +1 617 555 0100. Treated by Gregory House.",
        [
            ("Emily Davis", "PERSON"),
            ("MED-8877112", "PATIENT_ID"),
            ("+1 617 555 0100", "PHONE"),
            ("Gregory House", "PERSON")
        ],
        "medical_data"
    ),

    # ── Category 4: Financial Data ──
    (
        "Account Statement for Frank Miller. Account number: 9988776655. IBAN: DE89 3704 0044 0532 0130 00. Credit Card used: 4111 2222 3333 4444. Tax ID: DE123456789.",
        [
            ("Frank Miller", "PERSON"),
            ("DE89 3704 0044 0532 0130 00", "IBAN"),
            ("4111 2222 3333 4444", "CREDIT_CARD"),
            ("DE123456789", "TAX_ID")
        ],
        "financial_data"
    ),
    (
        "Invoice INV-2026-908. Bill to: Société Générale, account 4532015112830366, SWIFT SOGEFRPP. Approved by John Smith. Transfer EUR 84,293 to IBAN FR76 3000 6000 0112 3456 7890 189.",
        [
            ("John Smith", "PERSON"),
            ("FR76 3000 6000 0112 3456 7890 189", "IBAN"),
            ("4532015112830366", "CREDIT_CARD")
        ],
        "financial_data"
    ),

    # ── Category 5: Developer Workflows ──
    (
        "2026-06-21 12:00:00 [ERROR] Connection failed to dev-db.internal.company.com:5432 using postgres://admin:supersecretpassword@10.0.0.5/dbname. API key: api_key_abcd1234efgh5678.",
        [
            ("dev-db.internal.company.com", "HOST"),
            ("supersecretpassword", "CREDENTIAL"),
            ("api_key_abcd1234efgh5678", "SECRET"),
            ("postgres://admin:supersecretpassword@10.0.0.5/dbname", "URL")
        ],
        "developer_workflows"
    ),
    (
        "Please use the following AWS configuration:\naws_access_key_id = AKIAIOSFODNN7EXAMPLE\naws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY\ngithub_token = ghp_11223344556677889900aabbccddeeffgghh",
        [
            ("AKIAIOSFODNN7EXAMPLE", "SECRET"),
            ("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "SECRET"),
            ("ghp_11223344556677889900aabbccddeeffgghh", "SECRET")
        ],
        "developer_workflows"
    ),

    # ── Category 6: Adversarial Inputs ──
    (
        "Reach me at john [dot] smith [at] example [dot] com or call +1-202-555-o199.",
        [
            ("john [dot] smith [at] example [dot] com", "EMAIL"),
            ("+1-202-555-o199", "PHONE")
        ],
        "adversarial_inputs"
    ),
    (
        "My name is j0hn sm1th and my password is p4ssw0rd!",
        [
            ("j0hn sm1th", "PERSON"),
            ("p4ssw0rd", "CREDENTIAL")
        ],
        "adversarial_inputs"
    ),
    (
        "Na1ne: Al1ce S1n1th, Erna1l: al1ce@exan1ple.corn, Phor1e: 555_0123",
        [
            ("Al1ce S1n1th", "PERSON"),
            ("al1ce@exan1ple.corn", "EMAIL"),
            ("555_0123", "PHONE")
        ],
        "adversarial_inputs"
    )
]

dataset = []
for idx, (text, entities_raw, category) in enumerate(raw_samples):
    entities = []
    for substr, etype in entities_raw:
        # Find all occurrences of substr
        start = 0
        while True:
            pos = text.find(substr, start)
            if pos == -1:
                break
            # Add entity
            entities.append({
                "type": etype,
                "start": pos,
                "end": pos + len(substr)
            })
            start = pos + len(substr)
            
            # For our test dataset, let's assume one match per text (or process all)
            break
            
    # Verification
    for ent in entities:
        sliced = text[ent["start"]:ent["end"]]
        assert sliced in [x[0] for x in entities_raw], f"Error: slice '{sliced}' does not match original substring!"

    dataset.append({
        "id": idx + 1,
        "category": category,
        "text": text,
        "entities": entities
    })

os.makedirs("datasets", exist_ok=True)
with open("datasets/golden_dataset.json", "w") as f:
    json.dump(dataset, f, indent=2)

print(f"Generated {len(dataset)} golden dataset records in datasets/golden_dataset.json")
