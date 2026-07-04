# TUIML style guide

TUIML ("TUI Markup Language") is the name for tuigo's declarative
composition style. It is not literal markup — there is no parser, no JSX,
no template language. TUIML *is* Go: a component is a function that returns
an `Element` built from `Box`, `Text`, `Fragment`, and functional options.

```go
Box(
    Padding(1), FlexColumn(), Border(BorderSingle),
    Children(
        Text("count: %d", n),
        With(Text("press q to quit"), Italic()),
    ),
)
```

## Rules

1. **No JSX, no markup parsing.** Composition is plain Go function calls
   returning `Element`. If you find yourself wanting to parse a string into
   a tree, stop — add a constructor or option instead.
2. **Element construction is pure data.** Constructors and options never
   mutate an `Element` passed in from outside; each call returns an
   independent tree fragment. `With` copies before mutating for this reason.
3. **Layering stays pure → impure.** `types` (data) → `layout` (pure
   geometry) → `renderer` (pure cell buffer + diff) → terminal I/O (impure
   shell, always last). A lower layer never imports a higher one.
4. **Style inheritance.** Text-styling fields (`FgColor`, `BgColor`, `Bold`,
   `Italic`, `Underline`) cascade from an ancestor `Box` down to `Text`
   children that don't set their own value. Layout-affecting fields
   (`Padding`, `Margin`, `Gap`, `Border`, `Width`/`Height`, `FlexDir`) never
   inherit — they only ever apply to the node they're set on.
5. **Options apply left-to-right, deterministically.** `Box(A, B, C)` must
   behave the same every run; later options override earlier ones when they
   touch the same field.
6. **Every pure package ships table-driven tests.** No behavior lands
   without a test demonstrating it — see `PLAN.md`'s Testing Plan.

## Color guidelines

- **Prefer the named theme colors** (`Theme.TextPrimary`, `Theme.Surface`,
  etc.) for all application UI. The theme ensures visual consistency.
- **Use raw palette colors** (`ColorRed`, `ColorBlue`, etc.) only for
  quick prototyping or when you need a specific xterm shade.
- **Use `RGB(r, g, b)` for one-off visual effects** that don't belong in
  the theme — a distinctive highlight, a debug color, a user-defined accent.
  RGB colors are 24-bit; they require a truecolor-capable terminal.

## Rules from TODO.md items 1–3

7. **Give lists of components explicit keys.** Positional fallback identity
   only works while sibling order and count are stable; anything that can
   reorder or resize needs `WithKey`.
8. **Events are focus-scoped by default.** Use `Global()` deliberately and
   sparingly — for app-wide shortcuts (quit, help), not as a way to avoid
   thinking about focus.
9. **Only the render loop goroutine touches state.** Anything triggered
   from another goroutine (timers, future async I/O) must go through a
   channel into the loop, never call a setter directly.

## Adding a new style option

1. Add the field to `types.Style` (`types/style.go`) if it doesn't exist.
2. Add a constructor in `style.go` at the root: `func Foo(v T) Option`.
3. If the field cascades (text-styling), document that in its doc comment
   and wire the cascade in the layout/render pass that resolves styles.
4. Add a table-driven test asserting the option sets exactly that field and
   leaves the rest of `Style` untouched.
