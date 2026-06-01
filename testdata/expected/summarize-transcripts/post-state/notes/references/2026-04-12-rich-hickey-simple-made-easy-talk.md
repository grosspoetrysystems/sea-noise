---
author: Rich Hickey
consumed: false
date: 2026-04-12
description: Strange Loop talk on the difference between simple and easy
published: 2011-10-20
source: https://www.youtube.com/watch?v=SxdOUGdseq4
tags:
- reference
- clippings
- tech/software-design
title: Simple Made Easy, by Rich Hickey
---

Strange Loop keynote from Rich Hickey. The core distinction: *simple* (sim-plex — one fold/twist) is objective, structural, about the thing itself. *Easy* (adjacent, near at hand) is subjective, relative to where you are right now. Conflating them produces software that feels productive in the moment and rots underneath. Includes the famous knit/braid/cardigan analogy and a long catalog of constructs sorted along the simple/complex axis: state vs values, methods vs functions, vars vs managed refs, inheritance vs polymorphism à la carte, syntax vs data, conditionals vs rules.

## Related

- [[2026-04-08-out-of-the-tar-pit-paper]] — Moseley & Marks land in the same place from a different angle
- [[2026-04-15-worse-is-better-essay]] — the other side of the trade-off

## Research Context

### Key Takeaways

- **"Simple" and "easy" are different words solving different problems.** Simple (Latin *simplex*, "one fold") is objective — it describes structure and the absence of interleaving. Easy (Latin *adjacens*, "lying nearby") is subjective and relative to your skills, tools, and familiarity.
- **Simple is about lack of interleaving, not lack of cardinality.** You can have many simple things; simplicity is about each thing having one role, task, or concept — not about having fewer things.
- **We reach for "easy" under deadline pressure and accidentally ship complexity.** The familiar, nearby thing is often complex, and that complexity outlives the deadline that justified it — you live with it for the whole lifetime of the program.
- **Familiarity ≠ simplicity.** Just because something is close at hand and lets you "see a result fast" doesn't make it simple; that feeling is productivity theater that can mask braided-together design.
- **Hickey's simple-vs-complex inventory of everyday constructs:** state, methods, vars, inheritance, syntax, conditionals, and ORM are *complex* (they braid concerns together); values, functions, managed references, polymorphism à la carte, data, rules, and declarative manipulation are *simple*.
- **State is complex because it braids value, time, and identity** — a recurring theme in Hickey's thinking and a core motivation behind Clojure's design.
- **Simple is not always easy.** Some simple constructs are genuinely hard to use, especially without experience — which is precisely why developers avoid them despite their long-term payoff.

### Follow-up Questions

- If "easy" shifts per person and over time while "simple" stays fixed, how should a *team* make design decisions — optimize for what's simple objectively, or for what's easy for the current members?
- How would you actually *measure* the interleaving Hickey describes, so "this is complex" becomes a defensible claim rather than an aesthetic preference?
- Hickey says complexity outlives the deadline that produced it — what organizational incentives could make the long-term cost of "easy" choices visible at the moment they're made?
- Are there cases where deliberately choosing the complex-but-easy construct is the *correct* engineering trade-off, and how would you recognize one?
- If state, methods, and inheritance are complex, why have they remained the dominant constructs in mainstream languages — is it inertia, ecosystem familiarity, or do they offer something the "simple" alternatives don't?

### Related Materials

- **"Simple Made Easy" — original talk, Rich Hickey, Strange Loop 2011** — the canonical full version of this material. https://www.infoq.com/presentations/Simple-Made-Easy/
- **"The Value of Values" — Rich Hickey** — expands on why values are simple and state is complex. https://www.infoq.com/presentations/Value-Values/
- **"Are We There Yet?" — Rich Hickey** — his deeper treatment of state, identity, time, and values. https://www.infoq.com/presentations/Are-We-There-Yet-Rich-Hickey/
- **Clojure** — the language built around many of these simplicity principles. https://clojure.org/
- **"Out of the Tar Pit" — Ben Moseley & Peter Marks (2006)** — a paper directly on complexity (essential vs. accidental, state) that strongly complements this talk. https://curtclifton.net/papers/MoseleyMarks06a.pdf

### Book & Podcast Recommendations

**Books**
- *A Philosophy of Software Design* — John Ousterhout — A modern, practical treatment of complexity and how it accumulates, echoing Hickey's "interleaving" theme with concrete design rules.
- *The Mythical Man-Month* — Frederick P. Brooks Jr. — Brooks' essay "No Silver Bullet" introduces essential vs. accidental complexity, the conceptual backbone of distinguishing simple from easy.
- *Thinking, Fast and Slow* — Daniel Kahneman — Illuminates why we reach for the "nearby/familiar" under pressure, grounding Hickey's point about easy choices in cognitive bias.

**Podcasts**
- *Functional Geekery* (host Steven Proctor) — Interviews across functional programming that repeatedly return to simplicity, immutability, and Clojure-adjacent ideas.
- *Software Engineering Radio* — Long-running, depth-first interviews on design and architecture; good for episodes on complexity management and language design trade-offs.
- *CoRecursive* (host Adam Gordon Bell) — Story-driven episodes on programming language design and the reasoning behind tools like Clojure, well-matched to Hickey's framing.

## Transcript

Hi, my name's Rich Hickey, I'm the author of Clojure and I work for Cognitect. Today I want to talk about simplicity. And it might seem like an odd thing to talk about, because simplicity sounds easy, right? Like, why are we even here. But I think simplicity is one of the most important things we have to think about as software developers, and one of the most misunderstood. So I want to spend some time pulling these two words apart, because they get used interchangeably and it causes us real problems.

Let's start with the words themselves. Simple comes from the Latin sim-plex, which means one fold or one twist or one braid. The opposite of simple is complex, which means braided together, many twists, many folds. So simple is an objective notion. Something is simple or it isn't, based on its structure, based on what it is, based on the relationships between its parts. It doesn't matter who's looking at it. A thing has one role, or one task, or one concept, or one dimension. It's about lack of interleaving, not lack of cardinality. You can have a lot of simple things.

Easy is a totally different word. Easy comes from the Latin adjacens, lying nearby. Easy is relative. Something is easy with respect to where you are, with respect to what you already know, with respect to your tools, with respect to your experience. So easy is subjective, and it changes over time, and it changes per person. The thing you find easy this year, you might find easier next year, or you might find harder if you stop doing it. The same construct can be easy for one person and hard for another.

Now here's where we get into trouble as developers. We reach for the easy thing because we're under time pressure. We have a deadline, we have a feature to ship, we have a manager asking when it's going to be done. So we reach for the thing nearest at hand, the thing we already know, the thing that lets us type something into the editor and see a result. And we call that being productive. But what we've often done is reach for something complex, because it's easy. It's nearby. It's familiar. And we've put complexity into the program that we now have to live with for the lifetime of that program, which is much longer than the lifetime of the deadline that made us reach for it.

I want to do an inventory of constructs we use every day and sort them along the simple-versus-complex axis, ignoring whether they're easy or hard. So: state is complex, because state braids together value and time and identity. Values are simple. Methods are complex, because they braid together function and state and namespace. Functions are simple. Vars are complex; managed references are simple. Inheritance is complex; polymorphism à la carte is simple. Syntax is complex; data is simple. Conditionals are complex; rules are simple. ORM is complex; declarative data manipulation is simple. None of this is about easy. Some of the simple things are not easy to use, especially if you don't have experience with them.