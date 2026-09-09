# Preference sensitivity check — v5

The same fictional storm choices were evaluated with four preferences, two independent samples each. Only the preference text changed: weather, logistics, skill (expert), pass access, available ski days, and the v5 advisor prompt stayed fixed. Replacing each preference string with a marker yields identical prompts across all eight runs; evidence hashes also match. No reference choice or expected verdict was passed to the model.

| Preference | Choice in both samples | Practical reason |
| --- | --- | --- |
| No stated terrain preference | Cedar Ridge, Wednesday | Open glades and a useful fresh-snow day with less uncertainty |
| Tree skiing | Cedar Ridge, Wednesday | Accessible glades outweigh the larger snowfall at a bowl-only option |
| Untouched powder, willing to wait | Summit Peak, Thursday | Accept a possible bowl-reopening delay for a chance at untracked accumulation |
| Groomed corduroy | Valley Glide, Wednesday | Grooming finished after the snow; deeper snowfall elsewhere is a drawback |

Preferences therefore change the actual resort/day and accepted tradeoffs in this controlled case, not merely the wording. All verdicts were Recommended; tier changes are not required when different suitable options are available. The tree choice matches the neutral choice, so that pair alone does not establish a causal choice change; the contrasting powder/groomer profiles do. The untouched-powder preference explicitly includes willingness to wait, so this checks the combined preference, not terrain and risk tolerance independently.

Both research and structured extraction retained the selected option and the main preference-based reason. The untouched-powder answers recognize the downside of losing the day to closed bowls and the Wednesday decision deadline before the alternative expires. One tree answer offers Summit Peak as a fallback despite its weaker terrain fit; the other keeps Cedar Ridge on Thursday. This is a residual fallback-quality issue, not evidence that the main preference was ignored.

The setup deliberately makes the options distinct. This is a development sensitivity check, not proof of reliability with subtle, conflicting, mixed or changing preferences. Existing overconfidence about powder, crowds and operations appears here too. We did not add numerical preference weights or change the production prompt based on this test.

Eight research calls and eight extraction calls completed STOP. Google Search was available as in production, but the fictional scenario instructed no external research; all runs reported zero searches. Estimated total cost: $0.1319 using the app's rates. The harness renders the actual frozen v5 template and uses the actual powder schema snapshot; it is not an end-to-end preference-storage or ingestion test.

The separate Go regression `TestPromptKeepsPreferencesAlongsideProfile` passed, confirming that structured profile preferences and separately saved hunt preferences both reach `buildPrompt`. `UserProfileFromCore` copies the preference string and `FormatProfileForPrompt` renders it. No production code, model, database or notifications were changed for this check.
