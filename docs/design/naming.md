# Product name

Status: draft M0. **No legal advice.** This page summarizes a web search. It does not replace a
trademark search by an expert in the registers (DPMA, EUIPO, TMview, USPTO) for Nice classes 9
(software) and 42 (software as a service). The owner makes the final choice
(OPEN-QUESTIONS).

## Result

- **Working name: Bothy.** No chat or AI product of this name found; the only hits in
  trademark registers are outside software.
- **Repositories with the same name:** On GitHub there are small projects named `bothy`, among
  them a terminal workspace and a console for a single computer, both with few stars
  (https://github.com/search?q=bothy&type=repositories, GitHub search retrieved 2026-10-11)
  [verified]. None is a team chat; the terminal workspace, however, is close to the terminal of
  the product. Consequence: the repository and the Go module path need a unique owner path,
  and the register search (below) includes these hits [derived].
- **Alternatives: Corvo, Gilde.**
- The name is configuration (`branding.product_name`, ADR 0044). In the code it appears only in a
  few listed places (Go module path, package names, binary `bothy`), so that a change after the
  trademark check stays cheap.

## Criteria

1. No collision with a product in the field of chat, collaboration or AI agents.
2. No well-known brand, no company name, no person's name, no tainted term.
3. Pronounceable and spellable in German and English.
4. An image of its own that fits the product.
5. Short enough for app icon captions.

## Candidates

All evidence by web search, retrieved 2026-10-10. "Risk" is an assessment [derived].

| Name | Finding | Risk | Result |
|---|---|---|---|
| **Bothy** | No chat app "Bothy" found on the web or in the App Store [verified, web search]. Small GitHub projects with the same name outside chat (see Result) [verified]. Trademark hits only outside software: "BOTHY BLENDS" for spirits, UK, class 33 (https://www.ipo.gov.uk/t-tmj/tm-journals/2026-007/UK00004321045.html) and "BOTHOY" for hardware, US, class 9 (https://trademark.justia.com/885/15/bothoy-88515994.html) [verified]. Meaning: open shelter for everyone (https://en.wikipedia.org/wiki/Bothy) [verified]. | low | Working name |
| **Corvo** | No chat or AI app found [verified, web search]. Name of the main character of a game series (https://en.wikipedia.org/wiki/Corvo_Attano) [verified]; similar in sound to "Korvo", a local AI workspace (https://www.everydev.ai/tools/korvo) [verified]. | low to medium | Alternative 1 |
| **Gilde** | Software vendor "Gilde Software" in the Netherlands (https://www.g2.com/sellers/gilde) [verified]; the English equivalent is used by a platform for AI agents (https://docs.guild.ai/) [verified]. Immediately understandable in German. | medium | Alternative 2 |
| Plenum | well-known open-source repository of the same name (https://github.com/hyperledger-indy/indy-plenum) and a CMS interface "Plenum UI" (https://agilitycms.com/resources/posts/what-is-plenum-ui) [verified]; an IT consulting company of the same name according to a search summary [assumed] | medium | secondary |
| Heinzel | funded research project "Heinzel AI" in the field of AI helpers (https://www.uni-bonn.de/en/news/188-2025) [verified] | medium to high | rejected |
| Kobold | widespread open-source AI chat tools of the same name (https://github.com/LostRuins/koboldcpp) and a product brand of a household appliance maker held for decades (https://www.vorwerk.com/de/de/c/home/allgemein/presse/pressemitteilungen-kobold/90-jahre-schmutz-bekaempfer) [verified] | high | rejected |
| Tandem | team office app of the same name (https://apps.apple.com/app/id1521413406) and a large language app of the same name (https://apps.apple.com/app/tandem-learn-languages/id959001619) [verified] | high | rejected |
| Rudel | in the German-speaking world associated with a historically tainted person [verified, encyclopedia entry; reference deliberately not included] | high (reputation) | rejected |

Domains: All checked short `.dev` and `.app` domains of the eight words were registered in DNS
(DNS query `dig NS`, 2026-10-10) [verified]; an NXDOMAIN answer does not reliably mean free. Plan:
work with an addition (for example a word plus "chat" or "app"), check whether it can be bought
only at the registrar [derived].

In the field "team chat with AI agents as members" there are already vendors that advertise
"agentic team chat" (https://glue.ai/blog/how-agentic-team-chat-is-changing-the-way-we-work)
[verified]; the name should clearly stand apart from them [derived].

## Why Bothy

- **Image:** a simple hut, open to everyone, where you find shelter and work. The product is
  the place where people and agents work together, self-hosted like a hut of your own.
- **"bot" is in the word**, without the name sounding technical.
- **Design:** The palette (spruce, gorse, moss, rowan, granite) and the mark (a gable)
  follow the image; see [README.md](README.md).
- **Drawback:** The "th" is unfamiliar to German speakers; many will say "Botti"
  [assumed, no pronunciation test done].

## Next steps (owner)

1. Register search in classes 9 and 42 for Bothy, Corvo, Gilde by an expert.
2. Choose a domain with an addition and check it at the registrar.
3. Pronunciation test with a few German-speaking people.
4. Record the decision in DECISIONS.md; on a change, change only `branding.*`, the listed
   code places and the build configuration of the apps (ADR 0044).
