# Markdown Formatting Syntax

## Headings

To create a heading, add one to six `#` before your heading text. The number of `#` you use will deterimine the size of the heading.

```
# The largest heading

## The second largest heading

### The third largest heading

#### The fourth largest heading

##### The fifth largest heading

###### The sixth (and smallest) heading
```

# The largest heading

## The second largest heading

### The third largest heading

#### The fourth largest heading

##### The fifth largest heading

###### The sixth (and smallest) heading

Alternatively, for level 1 headings, you can add any number of `=` characters underneath a heading title.

```
The largest heading
===================
```

The largest heading
=

For level 2 headings, you can add any number of `-` characters underneath a heading title.

```
The second largest heading
--------------------------
```

The second largest heading
--------------------------

## Paragraphs

You can create a new paragraph by leaving a blank line between lines of text.

```
These two lines will
be combined into a single paragraph.

While this one will start a new paragraph.
```

These two lines will
be combined into a single paragraph.

While this one will start a new paragraph.

## Line Breaks

You can use two or more spaces for line breaks, or you can insert `<br>` at the end.

```
First line<br>
Second line
```

First line<br>
Second line

## Styling Text

You can indicate emphasis with bold or italics. For italics, use a single `*` or `_` before and after the text you wish to style.

```
This is *italic text*.
So is _this_.
But this is not.
```

This is *italic text*.
So is *this*.
But this is not.

For bold, use a double `*` or `_` before and after the text you wish to style.

```
This is **bold text**.
So is __this__.
But this is not.
```

This is **bold text**.
So is **this**.
But this is not.

For bold and italic, use a triple `*` or `_` before and after the text you wish to style.

```
This is ***bold & italic text***.
So is ___this___.
But this is not.
```

This is ***bold & italic text***.
So is ***this***.
But this is not.

You can also strike-through text by using a double `~` before and after the text you wish to strike-through.

```
This has ~~not~~ been struck through.
```

This has ~~not~~ been struck through.

## Block Quotes

You can quote text by preceding it with a `>`.

```
> Text that is quoted.

Text that is not quoted.
```

> Text that is quoted.

Text that is not quoted.

You can also nest the quoting by using multiple `>`.

```
> Text that is quoted.
>> Nested quoted text.
```

> Text that is quoted.
>> Nested quoted text.

### Alerts

You can also add various forms of alerts to the top of a block quote by starting it with the alert type:

```
> [!NOTE]
> Useful information that users should know, even when skimming content.

> [!TIP]
> Helpful advice for doing things better or more easily.

> [!IMPORTANT]
> Key information users need to know to achieve their goal.

> [!WARNING]
> Urgent info that needs immediate user attention to avoid problems.

> [!CAUTION]
> Advises about risks or negative outcomes of certain actions.
```

> [!NOTE]
> Useful information that users should know, even when skimming content.

> [!TIP]
> Helpful advice for doing things better or more easily.

> [!IMPORTANT]
> Key information users need to know to achieve their goal.

> [!WARNING]
> Urgent info that needs immediate user attention to avoid problems.

> [!CAUTION]
> Advises about risks or negative outcomes of certain actions.

## Quoting Code

You can call out code or a command within a sentence with single backticks. The text within the backticks will not be formatted.

```
Type `cd $HOME` to return to your home directory.
```

Type `cd $HOME` to return to your home directory.

To format code or text into its own distinct block, use triple backticks.

```
    ```
    Some text with
        its formatting preserved as typed.
    ```
```

```
Some text with
    its formatting preserved as typed.
```

## Links

Standard web links within the text are normally detected and converted automatically, such as <https://gurpscharactersheet.com>. You can, however, give them your own text to display by wrapping the link text in brackets `[ ]` and then wrapping the URL in parentheses `( )`.

```
Come visit [GCS](https://gurpscharactersheet.com)!
```

Come visit [GCS](https://gurpscharactersheet.com)!

You can also set the tooltip by adding some text wrapped in quotes `" "` after the URL.

```
Come visit [GCS](https://gurpscharactersheet.com "GURPS Character Sheet")!
```

Come visit [GCS](https://gurpscharactersheet.com "GURPS Character Sheet")!

## Images

You can display an image by adding `!` and wrapping the alt text in `[ ]`, then wrapping the URL in parentheses `( )`. The URL can also be a relative file path, as for links, above.

```
![GURPS Character Sheet](https://gurpscharactersheet.com/images/app_icon.svg)
```

![GURPS Character Sheet](https://gurpscharactersheet.com/images/app_icon.svg)

## Lists

You can make an unordered list by preceding one or more lines of text with `-`, `*`, or `+`.

```
- George Washington
- John Adams
- Thomas Jefferson
```

- George Washington
- John Adams
- Thomas Jefferson

To order your list, precede each line with a number.

```
1. George Washington
2. John Adams
3. Thomas Jefferson
```

1. George Washington
2. John Adams
3. Thomas Jefferson

You can also start the numbering at a different value.

```
4. See
5. This
```

4. See
5. This

Only the first number in the sequence is used. Subsequent ones are auto-incremented, so you can just repeat the same number for each line:

```
1. First
1. Second
1. Third
```

1. First
1. Second
1. Third

### Nested Lists

You can create a nested list by indending one or more list items below another item.

```
1. First list item
   - First nested list item
     - Second nested list item
   - More...
2. Another...
```

1. First list item
   - First nested list item
     - Second nested list item
   - More...
2. Another...

## Ignoring Markdown Formatting

You can tell the markdown to ignore formatting by using a `\` before the markdown character.

```
These asterisks \*will be preserved as-is\*.
```

These asterisks \*will be preserved as-is\*.

## Horizontal Rules

To create a horizontal rule, use three or more asterisks `***`, dashes `---` or underscores `___` on a line by themselves.

```
***
```

***

```
---
```

---

```
___
```

___

## Tables

To add a table, use three or more hyphens `---` to create each column's header and use pipes `|` to separate each column.

```
| First  | Second   |
|--------|----------|
| Line 1 | Column 2 |
| Line 2 | Column 2 |
```

| First  | Second   |
|--------|----------|
| Line 1 | Column 2 |
| Line 2 | Column 2 |

### Alignment within Tables

You can align text in the columns to the left, right or center by adding a colon `:` to the left, right, or on both sides of the hypens within the header row.

```
| Left | Right | Centered |
|:-----|------:|:--------:|
| aa   | bb    | cc       |
| aaaa | bbbb  | cccc     |
```

| Left | Right | Centered |
|:-----|------:|:--------:|
| aa   | bb    | cc       |
| aaaa | bbbb  | cccc     |

Note that tables don't have to have consistent column sizing.

```
| Left | Right | Centered |
|:---|---:|:---:|
| aa | bb | cc |
| aaaa | bbbb | cccc |
```

| Left | Right | Centered |
|:---|---:|:---:|
| aa | bb | cc |
| aaaa | bbbb | cccc |
