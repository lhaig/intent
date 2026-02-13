# What if programming languages were designed for AI, not humans?

A few weeks ago I saw a post on social media -- someone asking whether it was time to create a programming language designed for consumption by code assistants. Not for humans to write, but for AI to generate and humans to verify.

I couldn't stop thinking about it.

Every programming language I use was designed for humans to write. Python optimizes for readability. Rust optimizes for safety. JavaScript optimizes for getting something on screen fast. They all assume a human is typing the code.

But that's not really how I work anymore. Most of my code now starts with me describing what I want, an AI generating it, and me reviewing the result. The problem is that review step. When an AI generates 200 lines of Python, I have to read the *implementation* to know if it's correct. I'm tracing logic, checking edge cases, understanding control flow -- for code I didn't write, in a language optimized for *writing*, not for *reading someone else's work*.

That felt like the wrong tool for the job. So I started experimenting.

## The idea

What if the AI didn't just write code, but also wrote *contracts* -- formal statements about what the code guarantees? And what if the language made those contracts structural, not optional? Then instead of auditing implementations, you'd audit declarations:

```
function safe_divide(a: Int, b: Int) returns Result<Int, String>
    ensures result.is_ok() implies b != 0
    ensures result.is_err() implies b == 0
{
    if b == 0 {
        return Err("Division by zero");
    }
    return Ok(a / b);
}
```

Those two `ensures` lines tell you what this function guarantees. You don't need to read the body. The compiler generates runtime assertions that enforce the contracts -- if the implementation doesn't match, the program fails loudly rather than silently doing the wrong thing.

This is the core of what I've been calling **Intent** -- a contract-based language where the AI expresses guarantees, and the human verifies those guarantees match what they asked for.

## What makes it different from "just add asserts"

Fair question. A few things emerged as I worked on this that surprised me:

**Contracts aren't comments.** In Python or JavaScript, an assert is something you *can* add. In Intent, contracts are part of the grammar. The linter warns when they're missing. This matters because when an AI generates code, optional things tend to get skipped. Structural things don't.

**No type inference, on purpose.** `let x: Int = 5;` not `let x = 5;`. This felt wrong at first -- why make the AI write more? But the point isn't the AI's experience. It's the auditor's. When everything is explicit, there's nothing to mentally reconstruct.

**Intent blocks link English to code.** This is the feature I'm most curious about:

```
intent "Account safety" {
    goal "Maintain non-negative balances for all accounts";
    guarantee "Deposits always increase balance by exact amount";
    verified_by BankAccount.invariant;
    verified_by BankAccount.deposit.ensures;
}
```

The `verified_by` references are compiler-checked. If `BankAccount.deposit.ensures` doesn't exist, the program doesn't compile. You can't state a goal that isn't backed by an actual contract. I think there's something interesting here about bridging natural language and formal verification, though I'm still working out how far it can go.

## Where it is now

I built a working compiler in Go that produces native binaries via Rust. You can write programs with loops, arrays, enums, pattern matching, error handling (Result/Option with try operator), quantifier contracts (forall/exists), and multi-file modules with visibility control.

It's a proof of concept -- enough to test whether the idea has legs, not something you'd use in production. The biggest limitation is that contracts are only runtime-checked (assertions). The roadmap has Z3 SMT solver integration to *prove* contracts at compile time, which is where this would become genuinely more powerful than existing approaches. Today it's closer to "Rust with mandatory asserts" than it is to formal verification.

There's also no generics on user-defined types, no standard library, and no editor support. Though that last one matters less when the intended author is an AI, not a human in VS Code.

## Open questions

I'm sharing this because I think the question is more interesting than my particular answer to it. Some things I'm still thinking about:

- **Is "audit the contracts" actually faster than "audit the code"?** My experience says yes, but I'm biased. I'd like to see others try it.
- **What's the right level of contract detail?** Too little and you're back to trusting the implementation. Too much and auditing contracts is as hard as auditing code.
- **Should the AI also generate the intent blocks, or should those come from the human?** Right now the AI does everything. But maybe the human should write the intent blocks (goals and constraints in English) and the AI fills in the contracts and implementation.
- **How far can runtime assertions take you before you need static verification?** I was surprised how useful they were during development -- postcondition failures caught real bugs immediately. But for production use, you'd want compile-time proofs.

The code is open source. I'd genuinely like to hear what others think about the approach, the design tradeoffs, and whether this direction is worth exploring further.

**GitHub:** https://github.com/lhaig/intent


