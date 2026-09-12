## Evaluating the GO

## Programming Language with

## Design Patterns

by

### Frank Schmager

A thesis
submitted to the Victoria University ofWellington
in partial fulfilment of the
requirements for the degree of
Master of Science
in Computer Science.
Victoria University ofWellington
2010

### Abstract

GO is a newobject-oriented programming language developed at Google
by Rob Pike, Ken Thompson, and others. GO has the potential to become a
major programming language. GO deserves an evaluation.
Design patterns document reoccurring problems and their solutions.
The problems presented are programming language independent. Their
solutions, however, are dependent on features programming languages
provide.
In this thesis we use design patterns to evaluate GO. We discuss GO
features that help or hinder implementing design patterns, and present a
pattern catalogue of all 23 Gang-of-Four design patternswith GO specific
solutions.
Furthermore,we present GoHotDraw, a GO port of the pattern dense
drawing application framework JHotDraw. We discuss design and im-
plementation differences between the two frameworks with regards to
GO.

ii

# Acknowledgments

I would like to express my gratitude to my supervisors, James Noble
and Nicholas Cameron, whose expertise, understanding, and patience,
added considerably tomy graduate experience. They provided timely and
instructive comments and evaluation at every stage ofmy thesis process,
allowingme to complete this project on schedule.
I would also like to thankmy family for the support they providedme
throughmy entire life and in particular, I owemy deepest gratitude tomy
beautifulwifeMary,withoutwhose love andmoral support Iwould not
have finished this thesis.
Preliminary work for this thesis has been conducted in collaboration
withmy supervisors and published at the PLATEAU 2010workshop [83].
Frank Schmager
iii

iv

# Contents

1 Introduction 1
1.1 Contributions . . . . . . . . . . . . . . . . . . . . . . . . . . . 2
1.2 Outline . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . 2
2 Background 3
2.1 GO . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . 3
2.1.1 History of GO . . . . . . . . . . . . . . . . . . . . . . . 4
2.1.2 Overviewof GO . . . . . . . . . . . . . . . . . . . . . 5
2.2 Design Patterns . . . . . . . . . . . . . . . . . . . . . . . . . . 12
2.2.1 Design patterns vs Language features . . . . . . . . . 12
2.2.2 Client-Specified Self . . . . . . . . . . . . . . . . . . . 14
2.3 Language Evaluations . . . . . . . . . . . . . . . . . . . . . . 15
2.3.1 Language Critiques . . . . . . . . . . . . . . . . . . . . 15
2.3.2 Evaluation of Programming Languages for Students 15
2.3.3 Empirical Evaluations . . . . . . . . . . . . . . . . . . 17

2.3.4 Publications on GO . . . . . . . . . . . . . . . . . . . . 18
3 Design Patterns in GO 21
3.1 Why Patterns? . . . . . . . . . . . . . . . . . . . . . . . . . . . 21
3.2 Embedding or Composition . . . . . . . . . . . . . . . . . . . 23
3.2.1 Observer . . . . . . . . . . . . . . . . . . . . . . . . . . 23
3.2.2 Adapter . . . . . . . . . . . . . . . . . . . . . . . . . . 24
3.2.3 Proxy . . . . . . . . . . . . . . . . . . . . . . . . . . . . 24
v

vi CONTENTS
3.2.4 Decorator . . . . . . . . . . . . . . . . . . . . . . . . . 25
3.3 Abstract Classes . . . . . . . . . . . . . . . . . . . . . . . . . . 26
3.4 First Class Functions . . . . . . . . . . . . . . . . . . . . . . . 27
3.5 Client-Specified Self . . . . . . . . . . . . . . . . . . . . . . . . 29
3.5.1 TemplateMethod . . . . . . . . . . . . . . . . . . . . . 29
3.6 Information Hiding . . . . . . . . . . . . . . . . . . . . . . . . 31
3.6.1 Singleton . . . . . . . . . . . . . . . . . . . . . . . . . . 31
3.6.2 Fac¸ade . . . . . . . . . . . . . . . . . . . . . . . . . . . 32
3.6.3 Flyweight . . . . . . . . . . . . . . . . . . . . . . . . . 33
3.6.4 Memento . . . . . . . . . . . . . . . . . . . . . . . . . . 33
4 Case Study: The GoHotDraw Framework 35
4.1 Methodology . . . . . . . . . . . . . . . . . . . . . . . . . . . 36

4.2 The Design of GoHotDraw . . . . . . . . . . . . . . . . . . . . 37
4.2.1 Model . . . . . . . . . . . . . . . . . . . . . . . . . . . 39
4.2.2 View . . . . . . . . . . . . . . . . . . . . . . . . . . . . 43
4.2.3 Controller . . . . . . . . . . . . . . . . . . . . . . . . . 45
4.3 Comparison of GoHotDrawand JHotDraw . . . . . . . . . . 49
4.3.1 Client-Specified Self in the Figure Interface . . . . . . 49
4.3.2 GoHotDrawUser Interface . . . . . . . . . . . . . . . 51
4.3.3 Event Handling . . . . . . . . . . . . . . . . . . . . . . 55
4.3.4 Collections . . . . . . . . . . . . . . . . . . . . . . . . . 56
5 Evaluation 59
5.1 Client-Specified Self . . . . . . . . . . . . . . . . . . . . . . . . 59
5.2 Polymorphic Type Hierarchies . . . . . . . . . . . . . . . . . 60
5.3 Embedding . . . . . . . . . . . . . . . . . . . . . . . . . . . . . 61

5.3.1 Initialization . . . . . . . . . . . . . . . . . . . . . . . . 62
5.3.2 Multiple Embedding . . . . . . . . . . . . . . . . . . . 62
5.4 Interfaces and Structural Subtyping . . . . . . . . . . . . . . 63
5.5 Object Creation . . . . . . . . . . . . . . . . . . . . . . . . . . 64
5.6 Method and Function Overloading . . . . . . . . . . . . . . . 65

CONTENTS vii
5.7 Source Code Organization . . . . . . . . . . . . . . . . . . . . 65
5.8 Syntax . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . 66
5.8.1 Explicit Receiver Naming . . . . . . . . . . . . . . . . 66
5.8.2 Built-in Data Structures . . . . . . . . . . . . . . . . . 66
5.8.3 Member Visibility . . . . . . . . . . . . . . . . . . . . . 67
5.8.4 Multiple Return Values . . . . . . . . . . . . . . . . . 67
5.8.5 Interface Values . . . . . . . . . . . . . . . . . . . . . . 68
6 Conclusions 69
6.1 RelatedWork . . . . . . . . . . . . . . . . . . . . . . . . . . . 70
6.2 FutureWork . . . . . . . . . . . . . . . . . . . . . . . . . . . . 71
6.3 Summary . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . 72
A Design Pattern Catalogue 73
A.1 Creational Patterns . . . . . . . . . . . . . . . . . . . . . . . . 74

A.1.1 Abstract Factory . . . . . . . . . . . . . . . . . . . . . 74
A.1.2 Builder . . . . . . . . . . . . . . . . . . . . . . . . . . . 80
A.1.3 FactoryMethod . . . . . . . . . . . . . . . . . . . . . . 83
A.1.4 Prototype . . . . . . . . . . . . . . . . . . . . . . . . . 87
A.1.5 Singleton . . . . . . . . . . . . . . . . . . . . . . . . . . 90
A.2 Structural Patterns . . . . . . . . . . . . . . . . . . . . . . . . 94
A.2.1 Adapter . . . . . . . . . . . . . . . . . . . . . . . . . . 94
A.2.2 Bridge . . . . . . . . . . . . . . . . . . . . . . . . . . . 98
A.2.3 Composite . . . . . . . . . . . . . . . . . . . . . . . . . 102
A.2.4 Decorator . . . . . . . . . . . . . . . . . . . . . . . . . 107
A.2.5 Fac¸ade . . . . . . . . . . . . . . . . . . . . . . . . . . . 111
A.2.6 Flyweight . . . . . . . . . . . . . . . . . . . . . . . . . 115

A.2.7 Proxy . . . . . . . . . . . . . . . . . . . . . . . . . . . . 119
A.3 Behavioral Patterns . . . . . . . . . . . . . . . . . . . . . . . . 123
A.3.1 Chain of Responsibility . . . . . . . . . . . . . . . . . 123
A.3.2 Command . . . . . . . . . . . . . . . . . . . . . . . . . 127
A.3.3 Interpreter . . . . . . . . . . . . . . . . . . . . . . . . . 132

viii CONTENTS
A.3.4 Iterator . . . . . . . . . . . . . . . . . . . . . . . . . . . 137
A.3.5 Mediator . . . . . . . . . . . . . . . . . . . . . . . . . . 143
A.3.6 Memento . . . . . . . . . . . . . . . . . . . . . . . . . . 148
A.3.7 Observer . . . . . . . . . . . . . . . . . . . . . . . . . . 152
A.3.8 State . . . . . . . . . . . . . . . . . . . . . . . . . . . . 157
A.3.9 Strategy . . . . . . . . . . . . . . . . . . . . . . . . . . 162
A.3.10 TemplateMethod . . . . . . . . . . . . . . . . . . . . . 166
A.3.11 Visitor . . . . . . . . . . . . . . . . . . . . . . . . . . . 170
Bibliography 175

# Chapter 1

# Introduction

GO is a new programming languages developed at Google by Robert
Griesemer, Rob Pike, Ken Thompson, and others. GO was published
in November 2009 and made open source; was “Language of the year”
2009 [7]; and was awarded the Bossie Award 2010 for “best open source
application development software” [1]. GO deserves an evaluation.
Design patterns are records of idiomatic programming practice and
informprogrammers about good programdesign. Design patterns provide
generic solutions for reoccurring problems and have been implemented in
many programming languages. Every programming language has to solve
the problems addressed by patterns. In this thesiswe use design patterns
to evaluate the innovative features of GO.
In this thesiswe presents our experiences in implementing the design
patterns described inDesign Patterns [39], and the JHotDrawdrawing editor

framework in GO. We discuss differences between our implementations
in GO with implementations in other programming languages (primarily
Java)with regards to GO features.
1

2 CHAPTER 1. INTRODUCTION

### 1.1 Contributions

We present:
� an evaluation of the GO programming language.
� GoHotDraw, a GO port of the pattern dense drawing application
framework JHotDraw.
� a pattern catalogue of all 23 Design Patterns described in Design
Patterns [39].

### 1.2 Outline

The remainder of this thesis is organized as follows:
Chapter 2 gives background information concerning GO, design patterns,
and programming language evaluation.
Chapter 3 discusses selected design patternswith regards to GO’s language
features.
Chapter 4 describesGoHotDrawand highlights design and implementation
differences to its predecessor JHotDraw.
Chapter 5 evaluates GO.
Chapter 6 presents our conclusions, emphasizes this thesis’s original con-
tributions, and discusses possibilities for extending this work in the future.

# Chapter 2

# Background

In this chapterwe give background information necessary for understand-
ing the remainder of the thesis. In Section 2.1we describe the history of GO
—the languages and developers that influenced the design of GO—and
give necessary information about GO language features; in Section 2.2we
introduce design patterns; and in Section 2.3 we review existing literature
on language evaluation.

### 2.1 GO

GO is an object-oriented programming language with a C-like syntax. GO
is designed as a systems language, has a focus on concurrency, and is
“expressive, concurrent, garbage-collected” [4]. GO was developed by
Rob Pike, Ken Thompson and others at Google in 2007 [5] andwasmade
public and open source in November 2009. GO supports amixture of static
and dynamic typing, and is designed to be safe and efficient. A primary
motivation is compilation speed [70].
3

4 CHAPTER 2. BACKGROUND

### 2.1.1 History of GO

GO is inspired by many programming languages, environments, operating
systems and the developers behind them. Ken Thompson developed the B
programming language [50] in 1969, a predecessor of the C programming
language [82] developed by Dennis Richie in 1972. C is themain program-
ming language for the Unix operating system [55], the latter developed
initially by Thompson, Richie and others. Many of today’s languages have
a C-like syntax (C++, Java, C]).
Communicating Sequential Processes (CSP) [47]was developed by C.
A. R. Hoare in 1978. Hoare introduces the idea of using channels for
interprocess communication. CSP’s channels are unbuffered, i.e. a process
delays until another process is ready for either input or output on a channel.
CSP’s channels are also used as guards for process synchronization: a
process waits for multiple processes to finish by listening on multiple

channels for input and continues once a signal has arrived on a channel.
CSP’s approach to concurrency and interprocess communicationwas highly
influential for, amongst others, Occam[27] and Erlang [91].
In 1985 Luca Cardelli and Rob Pike developed Squeak [23], a language
to demonstrate the use of concurrency for handling input streams of user
interfaces. Squeakwas limited in that it didn’t have a type system, dynamic
process creation or dynamic channel creation. Pike redesigned Squeak as
Newsqueak in 1989 remedying Squeak’s limitations [69]. Newsqueakwas
syntactically close to C, had lambda expressions and the select statement
for alternations. Newsqueak introduced := for declaration-and-assignment
and the left arrow<- as communication operator (the arrowpoints in the
direction information flows) [74].
In 1992 Thompson, Winterbottom and Pike developed the operating

system Plan 9 [75]. Plan 9 was intended to become the Unix successor.
Plan 9waswidely usedwithin Bell Labs, and provided the programming
language Alef [95], developed by Winterbottom. Alef is a compiled C-
like language with Newsqueak’s concurrency and communicationsmodel.

2.1. GO 5
Unfortunately Alefwas not garbage collected and concurrencywas hard to
dowith the C-likememorymodel.
Winterbottom and Pike went on to develop Inferno [31] in 1995, a
successor to Plan 9. They also developed the Limbo programming language
[32] used to develop applications for Inferno. For Limbo,Winterbottom
and Pike reused and improved Alef’s abstract data types andNewsqueak’s
processmanagement [53].
Thompson and Pike’s experience and involvement in the development
of Unix, C, Plan 9, Inferno and Limbo have clearly influenced the design of
GO. GO draws its basic syntax fromC. GO’s interprocess communication
is largely influenced by CSP and its successors as outlined above. GO
uses typed channels which can be buffered and unbuffered; GO’s select
statement implements the process synchronizationmechanismdescribed

byHoare and implemented inNewsqueak. GO’s compiler suite is based
on Plan 9’s compiler suite. For declarations and modularity, GO drew
inspiration fromthe Pascal,Modula and Oberon programming languages
[96, 97], developed byWirth between 1970 and 1986.

### 2.1.2 Overview of GO

package main
import "fmt"
func main() {
fmt.Println("Hello World!")
}
The above listed GO programprints “HelloWorld!” on the console. The
package declaration is part of every GO file. GO’s syntax is C-like with
minor variations (e.g., optional semicolons, identifiers are followed by their
type). Functions are declaredwith the keyword func. The entry point to
every GO programis the function main in package main.
GO is systems programming languagewith focus on concurrency. Every

6 CHAPTER 2. BACKGROUND
function can be run concurrently with the keyword go. Concurrently
running functions are called goroutines. Goroutines are lightweight and
multiplexed into threads. Goroutines communicate via channels. GO is
garbage collected, and supports a mixture of static and dynamic typing.
GO has pointers, but no pointer arithmetic. Function arguments are passed
by value, except formaps and arrays.
Objects
GO is an object-oriented language. GO has objects, and structs rather than
classes. The following code example defines the type Car with a single
field affordablewith type bool:
type Car struct {
affordable bool
}
Methods
GO distinguishes betweenmethods and functions. Methods are functions
that have a receiver, and the receiver can be any valid variable name. Every
type in GO can havemethods. Following C++, rather than Java,methods

in GO are defined outside struct declarations. Multiple return values are
allowed and may be named; the receiver must be explicitly named. GO
does not support overloading ormulti-methods.
The following listing defines themethod Refuel for Car objects:
func (this Car) Refuel(liter int) (price float) {...}
Note that themethod Refuel is defined for type Car. Objects of pointer
type Car do not implement this method, the base type and the pointer
*
type are distinct. However, if a method is defined for objects of pointer
type, object of the base type automatically have themethod too.

2.1. GO 7
Embedding
GO has no class-based inheritance, code reuse is supported by embedding.
The embedding type gains the embedded type’smethods and fields. Types
can be embedded in structs by omitting the identifier of a field. For example,
the following code shows a Truck typewhich embeds the Car type and,
therefore, gains themethod Refuel:
type Truck struct {
Car
affordable bool
}
If a method or field cannot be found in an object’s type definition,
then embedded types are searched, andmethod calls are forwarded to the
embedded object. This is similar to subclassing, the difference in semantics
is that an embedded object is a distinct object, and further dispatch operates
on the embedded object, not the embedding one.
Objects can be deeply embedded andmultiply embedded. Name con-
flicts are only a problemif the ambiguous element is accessed. The example
above could bewritten as:

type Truck struct {
car Car
affordable bool
}
Here, Truck does not embed Car, but has an element car of type Car.
Truck could provide a Refuelmethod andmanually delegating the call
to car:
func (this *Truck) Refuel(liter int) (price float) {
return this.car.Refuel(liter)
}
Interfaces
Interfaces in GO are abstract representations of behaviour—sets ofmethod
signatures. An object satisfies an interface if it implements themethods of

8 CHAPTER 2. BACKGROUND
an interface. This part of the type systemis structural: no annotation by the
programmer is required. Interfaces are allowed to embed other interfaces:
the embedding interface will contain all the methods of the embedded
interfaces. There is no explicit subtyping between interfaces, but since type
checking for interfaces is structural and implicit, if an object implements
an embedding interface, then it will always implement any embedded
interfaces. Every object implements the empty interface interface{};
other, user-defined empty interfacesmay also be defined. The listing below
defines Refuelable as an interfacewith a singlemethod Refuel().
type Refuelable interface {
Refuel(int) float
}
Both Car and Truck implement this interface, even though it is not
listed in their definitions (and indeed, Refuelablemay have been defined

long afterwards). Type checking of non-interface types is not structural: a
Truck cannot be usedwhere a Car is expected.
Dynamic Dispatch
Dynamic dispatch andmethod calls with embedding in GO work differ-
ently thanwith inheritance in other languages.
Consider the classes S and T, where S extends T. T defines the public
methods Foo and Bazwith Baz calling Foo. The listing belowshows class
T in pseudo-code:
class T {
void Baz() {
this.Foo()
}
void Foo() {
print("T")
}
}

2.1. GO 9
The class S is a sub-class of T. S inherits Baz and overrides Foo:
class S extends T {
void Foo() {
print("S")
}
}
With class based inheritance calling Baz on an object of type Swill print
“S”. The dynamic type of s is S, so Bazwill call S’s Foomethod:
T s = new S();
s.Baz() //prints S
Embedding is GO’s mechanism for code reuse. The dispatch of mes-
sage calls with embedding is different fromthat of class-based inheritance.
Consider the types S and T,with S embedding T:
type T struct {}
type S struct {
T
}
We define themethod Baz on type T:
func (this T) Baz() {
this.Foo()
}
Type S implements themethod Foo printing “S”:
func (this S) Foo() {
print("S")
}
Type S gains themethod Baz, because S embeds T. The call s.Baz()
prints “T”,whereaswith class based inheritance “S”would be printed:
s := new(S)
s.Baz() //prints T
Packages

Source code organization and information hiding in GO is defined with
packages. The source code for a package can be split over multiple files.
A package resides in its own folder. A folder should only contain a single

10 CHAPTER 2. BACKGROUND
package. Packages cannot be nested. Folders can be nested to group
packages, but the packages have to be imported and addressed individually.
Wildcards cannot be used for importing packages, every package has to be
listed individually.
Information hiding
GO provides two scopes of visibility: package and public. A package pri-
vatemember is only visible within a package. Client code residing outside
the package cannot access package privatemembers. Publicmembers can
be accessed by package-external clients. The visibility is defined by the
first letter of an identifier. A lower case fist letter of the identifier renders
themember package private, an upper case first letter renders themember
public.
Object Creation
GO objects can be created fromstruct definitionswith orwithout the new
keyword. Fields can be initialised by the user when the object is initialised,

but there are no constructors.
Consider type Twith twomembers a and b:
type T struct {
Name string
Num int
}
To initialize T’s members, composite literals can be used. A composite
literal is the type name followed by curly braces and returns an instance of
the type. The parameters between the braces refer to the typesmembers.
The parameters refer to themembers in order they are declared. Composite
literals to initializemembers can only be used within the same package or
on publicmembers.
An object of type T can also be createdwith the built-in function new().
new(T) allocates zeroed storage for a newobject of type T and returns a

2.1. GO 11
pointer to it. The created object is of pointer type T.
*
Newly created objects in GO are always initialised with a default value
(nil, 0, false, "", etc.). There are multiple ways to create objects. On
each line in the following listing, an T object is created and a pointer to
the new object stored in t. The ampersand & operator returns thememory
address of a value. In each case the fields have the following values:
t.Name == "" and t.Num == 0.
var t *T
t = new(T)
t = &T{}
t = &T{"",0}
t = &T{Name:"",Num:0}
Multiple assignment/Multiple return parameters
GO supportsmultiple assignment:
a, b := "hello", 42
andmultiple return values:
func Foo() (string, int) {...}
...
a, b := Foo()
This functionality can be used in functions andmethods or can be used
to check and get an object fromamap. In the following listing elements is

amap. Multiple assignment is used to get the element belonging to key.
The second parameter (isPresent) is to distinguish between returning a
nil object belonging to a key or because the key does not exist. Note that
the first return parameter could be nil even though the second is true,
since values stored in amap can be nil.
element, isPresent := elements[key]
if !isPresent {
...
}

12 CHAPTER 2. BACKGROUND

### 2.2 Design Patterns

In software engineering certain problems reoccur in different applications
and common solutions re-emerge. Patterns are a description of reoccurring
design problems and general solutions for the problem. Patterns originate
from work on urban design and building architecture from Christopher
Alexander [10]. In 1987,Ward Cunninghamand Kent Beck started first ap-
plying patterns to software design [13]. Software patterns became popular
with Design Patterns [39] by Gamma,Helm, Johnson and Vlissides—the
“Gang-of-Four” (GoF)—published in 1994. Design patterns are themost
well known kind of software patterns.
A pattern consists usually of a name; a concise, abstract description of
the problem; an example; details of the implementation; advantages and
disadvantages; and known uses.
The patterns described in Design Patterns are object-oriented design

patterns, showing the relationships and interactions between objects and
classes. The problems address by patterns are mostly programming lan-
guage independent, the solutions, however, differ depending on the fea-
tures of a programming language. The GoF patterns have been imple-
mented in many programming languages (e.g. Java [65], JavaScript [46],
Ruby [67], Smalltalk [11], C] [15]).
Design patterns are just one category of software patterns, other cat-
egories are: software architecture [22], concurrency and networking [84],
resource management [56] and distributed computing [21]. Even whole
pattern languages have emerged [26, 93, 61, 34, 60].

### 2.2.1 Design patterns vs Language features

Norvig discusses design patterns and how features of the Dylan program-
ming language canmake the patterns simpler or invisible [66]. He claims
that 16 of the 23 design patterns can be simplified, but his slides give
no more information about how he arrived at these numbers. Amongst

2.2. DESIGN PATTERNS 13
other featuresNorvig exploits Dylan’smultimethods, syntax abstraction
mechanisms and runtime class creation to implement design patterns.
Gil and Lorenz argue that patterns can be grouped according to how
far the pattern is frombecoming a language feature [42]. Their groups are
cliche´s (trivialmechanisms not worth implementing as a language feature),
idioms (patterns that have already become features of some languages) and
cadets (patterns that are notmature enough to be language feature yet).
Bosch states that programming languages should have facilities to sup-
port implementations of design patterns [17]. He identified problems
implementing design patterns in C++ and proposes an extended object
model to remedy the shortcomings.
Agerbo and Cornils are of the opinion that the number of design pat-

terns should beminimal [8]. They develop four guidelines for evaluating
patterns: design patterns should be: domain independent, not an applica-
tion of another design pattern, language independent, and not a language
construct. They analyse patterns and conclude that 12 of the 23 the Gang of
Four patterns fulfil their guidelines. In a later work they call these patterns
fundamental design patterns [9].
Bu¨nning et. al. address the problemthat pattern implementations can
be hard to trace and hard to reuse [20]. They propose the language PaL
that elevates patterns to first class elements. They believe patterns can be
encapsulated by class structures enabling reuse and traceability.
Chambers,Harrison and Vlissides discuss the relationship and impor-
tance of patterns, languages, and tools [24]. They agree that the role of

design patterns ismainly for communication between humans. They dis-
agree over howfar languages and tools should go in aiding programmers
to apply patterns.
Bishop focusses on the relationship of abstraction and patterns [16].
She implements a selection of patterns in C] to convey her point that the
higher-level the functionality used to implement the pattern, the easier
andmore understandable are the implementations of the patterns. Bishop

14 CHAPTER 2. BACKGROUND
argues for higher-level functionality in programming languages tomake
pattern implementationsmore straight forward.

### 2.2.2 Client-Specified Self

The Client-Specified Self pattern enables programmers to simulate dynamic
dispatch in languageswithout inheritance by replacingmethod sends to
method receivers withmethod sends to an argument [90]. The “self” refers
to Smalltalk’s pseudovariable self (this in Java, C++ or C]). The pattern
allows the sender to effectively change the value of the recipient’s self.
Consider the example given in Section 2.1.2. S embeds T. S overrides
T’s Foomethod. The embedded type T is actually an object of type T, the
call that has been forwarded to the embedded object stays in that object.
Themethod Baz is defined on type T and the receiver object of Baz is self.
self is of type T. The call self.Foo()will call T’s Foomethod, printing
“T”. To achieve the same behaviour as with class based inheritance, the
actual receiver object has to be passed as an additional parameter.

We define an interface I that both type S and T implement:
type I interface {
Foo()
}
The changes necessary are bold in the next listing:
func (self B) Baz(i I) {
i.Foo()
}
a.Baz(a)
The parameter i of interface type I is added and the receiver of the Foo
call is the object i, not self. The client explicitly defines the receiver of
followingmethod calls.

2.3. LANGUAGE EVALUATIONS 15

### 2.3 Language Evaluations

### 2.3.1 Language Critiques

In his letter “Go To Statement ConsideredHarmful” [30] EdsgerW.Dijkstra
critizises a single programming language feature: goto statements. He
argues that structural programming with conditional clauses and loops
should be used instead of excessively using goto statements. He bases
his critique on the need to be able to reason about the state of a program.
According to him, gotomakes such reasoning “terribly hard”.
In “Lisp: GoodNews BadNewsHowtoWin Big” [37] Richard P.Gabriel
discusses why Lisp is not as successful as C. Gabriel bases his argument
not only on language features, but he also includes the Lisp community as
contributing factor. Gabriel’s article gave rise to the idea that incremental
improvement of a program and programming language might be better
for their adoption and long termsuccess than trying to produce something
perfectly initially.

In “Why Pascal isNotMy Favourite Programming Language” [54] Brian
W. Kernighan points out deficiencies of the Pascal programming language.
He bases his observations on a rewrite of a non-trivial Ratfor (Fortran)
program in Pascal. Kernighan selectively chooses language features to
support his conclusion that Pascal “is just plain not suitable for serious
programming”.

### 2.3.2 Evaluation of Programming Languages for Students

In the late 80’s and early 90’s a change in programming languages taught
in undergraduate courses can be observed: the change fromprocedural to
object-oriented. The early reports found in literature do not givemuch in
terms of evaluation of which language to chose. Later reports develop and
drawon a rich set of language evaluation qualities suitable for teaching.
Pugh et al. were among the first to apply the object-oriented approach

16 CHAPTER 2. BACKGROUND
in their under-graduate courses [78]. Smalltalk was the language of choice,
because it had a rich library, forced students to program in an object-
oriented style, and came with an IDE. Others followed and described their
experiences using Smalltalk to teach object-oriented programming [86, 87].
Their reasoningwhy they chose Smalltalk for teachmatchesmostlywith
Pugh’s report (pure object-oriented, extensive library, IDE).
Decker and Hirschfelder present more pragmatic reasons why they
chose Object Pascal for teaching [29]. They argue that the underlying
concepts can be taught inmany languages. They needed a language that
supportedmultiple programming paradigms. They admit that C++ and
Smalltalkweremore popularwith employers, but the facultywas used to
Pascal.
None of the above reports present a comprehensive set of criteria un-

derlying their decision forwhich language to choose for teaching. Thiswas
also observed byMazaitiswho surveyed academia and industry concerning
the choice of programming language for teaching [63].
A relatively comprehensive evaluation of programming languages is
presented by Ko¨ lling et al. [57]. They give a list of 10 requirements for a
first year teaching language. They evaluate C++, Smalltalk, Eifel and Sather
and come to the conclusion that none of these languages are fit for teaching.
They go on and develop the programming language and IDE Blue [58, 59].
Further evaluations of programming languages follow: Hadjerrouit
evaluates Java with seven criteria [44]; Gupta presents 18 criteria and a
short evaluation of four languages (C, Pascal, C++, Java) [43]; Kelleher and
Pausch present a taxonomy of languages and IDE catered to novices from

languages used in industry like Smalltalk and COBOL to languages for
children like Drape or Alice [52]; Parker et. al propose a formal language
selection processwith an extensive list of criteria [68] .

2.3. LANGUAGE EVALUATIONS 17

### 2.3.3 Empirical Evaluations

An early experiment comparing two programming languages to determine
if statically typed or typeless languages affect error rates was conducted by
Gupta [40]. Two programming languages have been designed with similar
features: one statically typed and one typeless language. The subjects were
divided in two groups and had to implement a solution for the same prob-
lem first with one and then with the other programming language. The
number of errorsmadewhile developingwith either languagewere com-
paredwith each other. Gupta finds that using a statically typed language
can increase programming reliability.
Tichy argues that computer scientists should experimentmore—experi-
ments according to the scientificmethod [88]. He dismissesmain arguments
against experimentation by comparing computer science to other fields of

science by giving examples strengthening the need for experiments, and by
showing the shortcomings of alternative approaches.
Prechelt conducted a programming language comparisons by having
groups of programmers develop the same programin different languages
[76]. He compared Java with C++ in terms of execution time and mem-
ory utilization by comparing the implementations of the same program
implemented by 40 students. Gat repeated Prechelt’s experiment and had
the same program implemented by different programmers in Lisp [41].
Gat found that the Lisp programs execute faster than C++ programs, and
development time is lower in Lisp than in C++ and Java. He concludes
that Lisp is an alternative to Java or C++where performance is important.
Prechelt extends the experiment to compare seven programming languages
in [77] according to program length, programming effort, runtime effi-

ciency,memory consumption. Prechelt observed amongst other findings,
that designing andwriting the programin C, C++, or Java takes twice as
long aswith Perl, Python, Rexx, or Tcl;with no clear differences in program
reliabilty
McIver argues that languages should be evaluated in connectionwith

18 CHAPTER 2. BACKGROUND
an IDE by conducting empirical studies observing the error rate [64]. Her
evaluation is geared towards novice programmers.
In a more recent study Hanenberg studies the “impact” of the type
systemon development [45]. He has one group of developers implement a
programin a statically typed language and the other group implementing
the same programin a dynamically typed language. He uses development
time and passed acceptance tests to measure the “impact”. Hanenberg
found no differences, in terms of development time, between static type
systems and dynamic type systems.

### 2.3.4 Publications on GO

In the first public introduction of GO [70] Rob Pike lays out reasons why
a newprogramming languagewas needed. Pike names goals for GO and
points out shortcomings of until then existing languages. According to Pike,
GO was needed because building software takes too long, existing tools are
too slow, type systems are “clunky”, and syntaxes are too verbose. GO is to
be type- andmemory safe, supporting concurrency and communication,
garbage-collected and quick to compile. He places emphasis on GO’s
concise, clean syntax, lightweight type systemand superior packagemodel
and dependency handling.
In another talk [71] Pike reasons about shortcomings of existingmain-
streamlanguages: mainstreamlanguages have an “inherent clumsiness”;
C, C++ and Java are “hard to use”, “subtle, intricate, and verbose”; and
“their standard model is oversold and we respond with add-on models

such as ‘patterns’ ”. Pike points out howlanguages like Ruby, Python, or
JavaScript react to these problems. He identifies a niche for a newprogram-
ming language and shows how GO tries to fill that niche with qualities
that a language has to have to avoid deficiencies of existing languages. He
explains GO concepts and functionality like interfaces, types, concurrency.
Pike concludes by presenting achievements and testimonials of GO users.

2.3. LANGUAGE EVALUATIONS 19
At OSCON 2010 Pike gave two presentations on GO. The first talk
lays out GO’s history, the languages that influenced GO’s approach to
concurrency [74]. The second talk [72] is another overview of GO’s type
system (making comparisons with Java); GO’s approach to concurrency
andmemorymanagement; GO’s development status.
In themost recent presentation of GO [73] Pike points out that GO is a
reaction to Java’s and C++’s “complexity,weight, noise” and JavaScript’s
and Python’s “non-static checking”. He backs up his claim that GO is
a simple language, by comparing the number of keywords in various
languages (with GO having the least). He gives the same examples of
earlier talks and focusses onmainly the same functionality (interfaces and
types, and concurrency).

20 CHAPTER 2. BACKGROUND

# Chapter 3

# Design Patterns in GO

Design patterns are records of idiomatic programming practice and inform
programmers about good programdesign. Design patterns have been the
object of active research for many years [26, 92, 35, 85]. We use design
patterns to evaluate the GO programming language. In this Chapter we
present patterns fromDesign Patterns [39]which highlight the innovative
features of GO.
In Section 3.1 we discuss why we are using patterns to evaluate GO;
in Section 3.2we discuss patternwhere either embedding or composition
should be used; in Section 3.3 we discuss patterns that rely on structural
subtyping; in Section 3.4we discuss patterns that can be implementedwith
GO’s first class functions; in Section 3.5we discuss howapplying the Client-
Specified Self pattern affects the TemplateMethod pattern; in Section 3.6
we discuss patterns thatmake use of GO’s approach to information hiding.

### 3.1 Why Patterns?

Many researchers have proposedmethods and criteria for evaluating pro-
gramming languages (see Section 2.3). Themethods and criteria proposed
in the literature are usually hard tomeasure and are prone to be subjective.
Design patterns encapsulate programming knowledge. The intent and
21

22 CHAPTER 3. DESIGN PATTERNS IN GO
the problems patterns address, and furthermore the necessity for patterns,
are programming language independent. Implementations of design pat-
terns differ due to specifics of the language used. There is a need for
programming language specific pattern descriptions as can be seen from
the number of books published on the subject (see Section 2.2).
Gamma et al. say that the choice of programming language influences
one’s point of view on design patterns and that some patterns are sup-
ported directly by some programming languages. Thus, design patterns
don’t have to be programming language independent. Others argue that
design patterns should be integral parts of programming languages [17, 9].
Agerbo and Cornils analysed the GoF design patterns to determine which
patterns could be expressed directly, using features of a sufficiently power-

ful programming language [8, 9].
Patterns address common problems and provide accepted, good solu-
tions. Programming languages have to be able to solve these problems,
either by supporting solutions directly or by enabling programmers to
implement the solution as a pattern.
We implemented the design patterns in GO to determine possible solu-
tions to the problems the patterns address. Our use of patterns should give
programmers insight into how GO-specific language features are used in
everyday programming, and howgaps, comparedwith other languages,
can be bridged. We think that using design patterns is a viable way to
evaluate a programming language. By using design patterns to evaluate
a programming languagewe give advanced programmers an insight into
language specific features, and enable novices to learn a new program-

ming languagewith familiar concepts. The implementations in different
programming languages could serve as the basis for further comparison.

3.2. EMBEDDING OR COMPOSITION 23

### 3.2 Embedding or Composition

In this section we highlight patterns where both embedding and compo-
sition are viable alternative design decisions. Furthermore we point out
patterns that should (or should not) be implementedwith embedding.

### 3.2.1 Observer

In theObserver pattern a one-to-many relationship between interesting and
interested objects is defined. When an event occurs the interested objects
receive amessage about the event.
Observers register with subjects. Subjects informtheir observers about
changes. Observers get the changes from the subject that notified them.
All subjects have operations in common to attach, detach and notify their
observes. (See Appendix A.3.7)
In the Design Patterns’ description of the Observer pattern, subject is
an abstract class. GO does not have abstract classes. The definition of an
interface and embedding can be used, even though it is not as convenient.
Amajor advantage of embedding becomes apparent in the implementa-
tion of the Observer pattern. Every type can bemade observable, only a
type implementing the subject’s interface needs to be embedded. In Java,

this is not possible. Java supports single class inheritance, which limits
the ability to inherit the observable functionality. In GO however,multiple
types can be embedded. Types that are doing their responsibility can easily
bemade observable. Embedding the observable functionality in subjects
works only as long as the subjects expect the same type of observer (e.g.
amouse only acceptingmouse observers). The embedding type can still
override the embedded type’smembers to provide different behaviour.
Even though the embeddingmakes it easy to implement the Observer
pattern, as far as we can see, the pattern is not as widely used in GO as it is
in Java libraries (Listeners in Swing for example).

24 CHAPTER 3. DESIGN PATTERNS IN GO

### 3.2.2 Adapter

The Adapter pattern enables programmers to translate the interface of a
type into a compatible interface. An adapter allows types to work together
that normally could not because of incompatible interfaces.
The solution is to provide a type (the adapter) that translates calls to its
interface (the target) into calls to the original interface (the adaptee). (See
Appendix A.2.1)
The adapter implements the target’s interface. The adaptingmethods
delegate calls of the target to the adaptee. Adapters can be implemented
as described in Design Patterns: the adapter maintains a reference to the
adaptee and delegates calls to that reference. GO provides an extended
fromof composition: embedding. The adapter could embed the adaptee.
The adapter still has to implement the target’s interface by delegating calls
of the target to the adaptee.

Having the choice between embedding and composition is similar to
class-based languageswhere an adapter can use inheritance or composition.
Composition should be used when the public interface of the adaptee
should remain hidden. Embedding exhibits the adaptee’s entire public
interface but reduces bookkeeping.

### 3.2.3 Proxy

The Proxy pattern allows programmers to represent an object that is com-
plex or time consuming to createwith a simpler one.
The proxy, or surrogate, instantiates the real subject the first time the
clientmakes a request of the proxy. The proxy remembers the identity of
the real subject, and forwards the instigating request to this real subject.
Then all subsequent requests are simply forwarded directly to the encap-
sulated real subject. Proxy and real subject both implement the interface
subject, so that clients can treat proxy and real subject interchangeably. (See
Appendix A.2.7)

3.2. EMBEDDING OR COMPOSITION 25
Proxy can be implemented in GO with object composition as described
in Design Patterns. In GO we also have the option to use embedding.
Instead of maintaining a reference to a subject, the proxy could embed a
real subject. The proxy would gain all of the subjects functionality and
could override the necessarymethods. The advantage of embedding is that
message sends are forwarded automatically, no bookkeeping is required.
The disadvantage of embedding a real subject is, that the proxy type cannot
be used dynamically for different real subjects. Each real subject typewould
need their own proxy type.

### 3.2.4 Decorator

The Decorator pattern lets programmers augment an object dynamically
with additional behaviour.
A decorator wraps the component, the object to be decorated, and adds
additional functionality. The decorator’s interface conforms to the com-
ponents’s interface. The decoratorwill forward requests to the decorated
component. Components can be decorated with multiple decorators. It
is transparent to clients working with components if the component is
decorated or not. (See Appendix A.2.4)
TheDecorator pattern in GO needs to be implementedwith composition.
Decorators should not embed the type to be decorated. The decorator
maintains a reference to the object that is to be decorated. If the decorator
embedded a component, every time a new component type were added a
new decorator type would be needed too. This would cause an “explosion”

of types,which the decorator pattern is trying to avoid. In a hypothetical
solution the decorator had to embed the component’s common interface,
but in GO only types can be embedded in structs, not interfaces.

26 CHAPTER 3. DESIGN PATTERNS IN GO

### 3.3 Abstract Classes

We describe in this section how the lack of abstract classes affects the
implementation of certain design patterns.
Composite The Composite pattern lets programmers treat a group of
objects in the same way as a single instance of an object and creates tree
structures of objects. The key is to define an interface for all components
(primitives and their containers) to be able to treat them uniformly. (See
Appendix A.2.3)
Bridge The Bridge pattern allows programmers to avoid permanent bind-
ing between an abstraction and an implementation to allowindependent
variation of each. (See Appendix A.2.2)
Design Patterns describes both pattern, Composite and Bridge,with ab-
stract classes: Composite in the Composite pattern and Abstraction in the
Bridge pattern.
GO has no abstract classes. To achieve the advantages of abstract classes

(define interface and provide common functionality) two GO concepts have
to be combined. An interface for the interface definition, and a separate
type encapsulating common functionality. Types wishing to inherit the
composite functionality have to embed this default type.
Like in Java, interfaces can be extended in GO too. This is done by
one interface embedding another interface. Interfaces can embedmultiple
interfaces. In the Composite pattern the composite interface embeds the
component interface ensuring that all composites are components too.

3.4. FIRST CLASS FUNCTIONS 27

### 3.4 First Class Functions

In this section we discuss patterns that take advantage of GO’s support for
first class functions.
Strategy The Strategy pattern decouples an algorithm from its host by
encapsulating the algorithminto its own type.
A strategy provides an interface for supported algorithmswhich con-
crete strategies implement. A contextmaintains a reference to a strategy.
The context’s behaviour depends on the dynamic type of the strategy and
can be changed dynamically. (See Appendix A.3.9)
State The State pattern lets you change an object’s behaviour by switching
dynamically to a set of different operations.
The object whose behaviour is to be changed dynamically is the context
andmaintains a state object. Asking the context to execute its functionality
will execute the context’s current state’s behaviour. The behaviour depends
on the dynamic type of the state object. (See Appendix A.3.8)

Command The Command patterns allows programmers to represent and
encapsulate all the information needed to call amethod at a later time, and
to decouple the sender and the receiver of amethod call. The Command
pattern turns requests (method calls) into command objects. An invoker
maintains a reference to a command object. Clients interactwith the invoker.
The invoker’s behaviour depends on the dynamic type of the command.
(See Appendix A.3.2)
The Strategy, State or Command pattern can be implemented as sug-
gested inDesign Patterns: the algorithmfor each strategy, state, command is
its own type encapsulating the specific behaviour. The states’ and strategies’
context and the commands’ invokermaintain references to the respective

28 CHAPTER 3. DESIGN PATTERNS IN GO
objects. The context’s/invoker’s behaviour depends on the dynamic type
of the objects. In GO however,we don’t need to define a separate type for
each algorithm. GO has first class functions. Strategies, states or commands
can be encapsulated in functions and the contexts/invokers store references
to functions, instead of objects.
Defining a type and amethod for each algorithmis elaborate compared
to defining a function. Strategies, states or commands could be Singletons
or Flyweights. Encapsulating algorithms as functions removes the necessity
for Singleton or Flyweight. However, if the algorithms need to store state,
they are better implemented with types. Functions could store state in
package-local static variables, but this would break encapsulation, since
other functions in the package have access to those variables.

Using objects to encapsulate the algorithms, the context’s/invoker’s
behaviour depends on the dynamic type of the objects. Figuring out which
method is called at a certain point can be a daunting task. Using functions
makes the code easier to follow.
There are advantages of having a typewithmethods over having func-
tions only. Objects can store state. Functions in GO can store state only
in static variables defined outside the function’s scope. Variables defined
inside the function’s scope cannot be static; they are initialized every time
the function is called. The problem with static variables is that they can
be altered externally too. The function cannot rely on the variable’s state.
Having a type allows programmers to use helpermethods and embedding
for reuse. This possibility is not given to functions. Subtypes could over-

ride the helper functions or override the algorithm and reuse the helper
functionality.
Encapsulation could be achieved by organizing each function in a sepa-
rate package, but this comes at a cost: each package has to be in a separate
folder, additionally the packages need to be imported before the functions
can be used. Thismight bemorework than implementing separate types.

3.5. CLIENT-SPECIFIED SELF 29

### 3.5 Client-Specified Self

Embedding does not support the usual object-oriented behaviour of dy-
namic dispatch on self. In particular, GO will dispatch fromouter objects
to inner objects (up from subclasses to superclasses) but not in the other
direction, frominner objects to outer objects (down fromsuperclasses to
subclasses). Downwards dispatch is often useful in general, and particu-
larly so in the TemplateMethod and FactoryMethod patterns. Downwards
dispatch can be emulated by passing the outermost “self” object as an extra
parameter to allmethods that need it—implementing the client-specified
self pattern (see Section 2.2.2). To use dynamic dispatch, the method’s
receivermust also still be supplied.

### 3.5.1 TemplateMethod

The TemplateMethod pattern enables the programmer to define a stable
outline of an algorithmand letting subclasses implement certain steps of
the algorithmwithout changing the algorithm’s structure.
The usual participants of TemplateMethod are an abstract class declar-
ing a finalmethod (the algorithm). The steps of the algorithmcan either be
concretemethods providing default behaviour or abstractmethods, forcing
subclasses to override them. GO however, does not have abstract classes
nor abstractmethods.
Consider the templatemethod PlayGame:
func (this *BasicGame) PlayGame(game Game,
players int) {
game.SetPlayers(players)
game.InitGame()
for !game.EndOfGame() {
game.DoTurn()
}
game.PrintWinner()
}
PlayGame is defined for type BasicGame. Steps like SetPlayers are to

30 CHAPTER 3. DESIGN PATTERNS IN GO
be implemented by subclasses of BasicGame (e.g Chess).
Themajor difference comparedwith standard implementations is that
wemust pass an object to the templatemethod twice: first as the receiver
this BasicGame and then as the first argument game Game. This is the
*
Client-Specified Self pattern (Section 2.2.2). BasicGame will be embedded
into concrete games (like Chess). Inside PlayGame, calls made to this
will callmethods on BasicGame; GO does not dispatch back to the embed-
ding object. If we wish the embedding object’smethods to be called, then
we must pass in the embedding object as an extra parameter. This extra
parametermust be passed in by the client of PlayGame, andmust be the
same object as the first receiver. Every additional parameter places a further
burden onmaintainers of the code. Furthermore the parameter has the po-

tentially for confusion: chess.PlayGame(new(Monopoly), 2)). Here
we pass an object of type Monopoly to the PlayGamemethod of the object
chess. This callwill not play Chess, butMonopoly.
The combination of BasicGame and the interface Game is effectively
making EndOfGame an abstractmethod. Game requires thismethod, but
BasicGame doesn’t provide it. Embedding BasicGame in a type and not
implementing EndOfGame will result in a compile time error when it is
used instead of a Game object.
The benefits of associating the algorithmwith a type allows for provid-
ing default implementations, using of GO’s compiler to check for unim-
plementedmethods. The disadvantage is that sub-types like Chess could
override the template method, contradicting the idea of having the tem-
platemethod fixed and subtypes implementing only the steps. In Java, the

PlayGamemethodwould be declared final so that the structure of the
game cannot be changed. This is not possible in GO because there is no
way to stop amethod being overridden.
In class-based languages, the BasicGame class would usually be de-
clared abstract, because it does notmake sense to instantiate BasicGame.
GO, however, has no equivalent of abstract classes. To prevent BasicGame

3.6. INFORMATION HIDING 31
objects being used, we do not implement allmethods in the Game interface.
Thismeans that BasicGame objects cannot be used as the client-specified
self parameter to PlayGame (because it does not implement Game).

### 3.6 Information Hiding

In this sectionwe look at patterns that take particular advantage of GO’s
approach to information hiding.

### 3.6.1 Singleton

The Singleton pattern ensures that only a single instance of a type exists
in a program, and provides a global access point to that object. (See Ap-
pendix A.1.5)
The Singleton pattern is usually implemented by hiding the singleton’s
constructor (using private scope in Java). In GO, constructors cannot
be made private, but we have two ways to limit object instantiation by
using GO’s package accessmechanisms. The first option is to declare the
singleton type’s scope package private and to provide a public function
returning the only instance. Clients outside the package can’t create in-
stances of the un-exported type directly, only through the public function.
The Singleton pattern is only effective for clients outside the package a
singleton is declared. Within the singleton’s package multiple instances

can be created, since GO does not have a private scope; in particular, this
might be done inadvertently by embedding the private type.
The other option is to use a package as the singleton. Packages cannot
be instantiated and package initialization is only done once. The singleton
packagemaintains its state in package private static variables and provides
public functions to access the private state. The first approach requires
a separate type for representing the singleton and a function controlling
instantiation; the approach using the package as the singleton is less com-

32 CHAPTER 3. DESIGN PATTERNS IN GO
plex, because instantiation does not need to be controlled and no separate
type is necessary. Both approaches need to define public accessors for the
singleton’s state.

### 3.6.2 Fac¸ade

The Fac¸ade pattern defines a high-level abstraction for subsystems, and
provides a simplified interface to a larger body of code tomake subsystems
easier to use.
Clients can be shielded froma system’s low-level details by providing a
higher-level interface. To hide the implementation of subsystems, a Fac¸ade
is defined to supply a unified interface. Most Clients interact with the
fac¸ade without having to know about its internals. The Fac¸ade pattern still
allows access to lower-level functionality for the few clients that need it.
(See Appendix A.2.5)
We have two options to implement the Fac¸ade pattern in GO. The first
option uses composition and follows closely the solution given in Design
Patterns: A fac¸ade type is declaredmaintaining references to objects of the
subsystems that are to be hidden. The additional option that GO offers is

to use a package to represent a fac¸ade. The subsystems aremaintained in
package static variables. Access to the subsystems is given through public
functions. Using a package does not require creating an fac¸ade object. The
package and its subsystems are initialized in the init function, which
is automatically called on import. The initialization could be done in a
separatemethod (initwould call thatmethod), thus allowing clients to
re-initialize the package.
The Fac¸ade pattern should not be implemented with embedding: com-
position should be used instead, because the purpose is to hide low-level
functionality and to provide a simple interface. A fac¸ade that embeds
subcomponents publishes the entire public interface of the subcomponents,
counteracting the pattern’s intent.

3.6. INFORMATION HIDING 33

### 3.6.3 Flyweight

The Flyweight pattern enables the programmer tominimizememory use
by sharing asmuch data as possible between similar objects; it is away to
use objects in large numbers when a simple repeated representation would
use an unacceptable amount ofmemory.
The solution is to share objects which have the same intrinsic state and
provide themwith extrinsic state when they are used. This will reduce the
number of objects greatly. A small number of objects stores a minimum
amount of data, hence Flyweight. (See Appendix A.2.5)
The implementation of the Flyweight pattern as described in Design Pat-
terns requires a factory to control creation of flyweight objects. In contrast
to the Singleton pattern, the flyweight type can be public. Flyweights that
need to be shared are created by using the flyweight factory. Flyweights
can be used unshared by instantiating themdirectly.

The additional flyweight factory type is overhead. A public function
could do the factory’s job instead. The existing flyweightswould be kept
in a package local static variable. Only the factory function grants package
external clients access to flyweights. Singleton’s limitation of only being
effective for package external clients is present in Flyweight aswell. If the
flyweight objects have to be shared, the flyweights’ type has to be package
private to avoid uncontrolled instantiation. The hiding of the type is only
effective for package external clients, since flyweights can be instantiated
uncontrollablywithin the flyweight’s package.

### 3.6.4 Memento

TheMemento pattern lets the programmer bring an object back to a previ-
ous state by externalizing the object’s internal state.
The object whose state is to be restored is called the originator. The
originator instantiates a memento, an object that saves the originator’s
current state. Clients asks the originator for mementos to capture the

34 CHAPTER 3. DESIGN PATTERNS IN GO
originator’s state. Client then use themementos to restore the originator.
(See Appendix A.3.6)
Mementos have two interfaces: a narrowinterface for clients and awide
one for originators. In C++ originator is a friend ofmemento gaining access
to the wide interface. The methods in the narrow interface are declared
public. In GO we can use visibility scopes to shield the memento from
clients and keep it accessible for originators. We define the wide interface
with public methods and the narrow one with package local methods.
Memento and originator should reside in same package and clients should
be external, to protectmementos frombeing accessed.

# Chapter 4

# Case Study: The GoHotDraw

# Framework

In this chapterwe present GoHotDraw: a GO port of JHotDraw, a frame-
work for building graphical drawing editor applications. We used JHot-
Draw as a baseline for comparison to evaluate how GO specifics influence
the design and implementation of GoHotDraw.
JHotDraw was developed by Erich Gamma for teaching purposes. It is
amature Java framework that is publicly available [38] and stillmaintained
by an open source community. JHotDrawitself is based on a long history
of drawing editor application frameworks, most notably HotDraw[48].
HotDrawwas originally developed byWard Cunninghamand Kent Beck
in the programming language Smalltalk [18]. HotDrawis a pattern-dense
framework, making use of the majority of the design patterns proposed
in [39] and other design patterns. JHotDrawis also influenced by ET++, a
C++ application framework,whichwas also developed by Erich Gamma

[94]. JHotDraw and its predecessors were developed and designed by
expert programmers. JHotDraw is stillmaintained by its community. As a
consequence JHotDraw is a very mature framework, and a good way to
evaluate a programming language.
We highlight and discuss design similarities and differences to JHot-
35

36 CHAPTER 4. CASE STUDY: THE GOHOTDRAWFRAMEWORK
Draw and GoHotDraw with regards to design patterns, and we point
out differences in the overall design and implementation details we came
across.
This chapter is organized as follows: in Section 4.1 we present our
methodology; in Section 4.2 we give a brief overview of GoHotDraw’s
design; in Section 4.3 we discuss differences between GoHotDraw and
JHotDrawwith regards to design and implementation.

### 4.1 Methodology

Porting an existing application into GO has the advantage of having a basis
for comparison. TheHotDrawframeworkswere chosen because of their
extensive use of design patterns and framework’smaturity. The decision
to port JHotDraw instead of HotDraw is based on the author’s preference
for Java over Smalltalk.
We started by studying relevant literature describingHotDraw’s [14, 18,
28, 36, 49] and JHotDraw’s [25, 51, 81] design to gain an understanding of
how the patterns fit into the framework and how the pattern relate to each
other. Furthermore we investigated the source code of JHotDraw [38] to
find the subset of functionality necessary to cover themain patterns and to
implement basic functionality (drawing, selecting,moving and resizing of
figures).
JHotDrawis quite large. As a guide JHotDrawv5.3 is about 15,000 lines

of code and v7.3 about 70,000 lines, including several complete applications
[38]. We concentrated on the essence of the framework’s functionality
(Drawings contains Views, Views contain Figures, Editors enable Tools to
manipulate Drawings). This functionality subset suffices to cover all the
key patterns in JHotDraw’s design.
We started at the core of the framework: the Figure interface and imple-
mented CompositeFigure and RectangleFigure. Next came the Drawing
and Viewinterfaceswith a concrete implementation each to allowgraphical

4.2. THE DESIGN OF GOHOTDRAW 37
output, followed by the Editor. The Tool hierarchywas developed last. We
wanted to gain a better understanding of the framework and of GO before
attempting to implement the resize functionality. Creation and dragging of
figures is trivial compared to resizing. Resizing involves the actual change
of the underlyingmodel to reflect the size changes as well as displaying
handles, the rectangular boxes that appearwhen a figure is selected.
The subset of functionality that we implemented amounts to around
2,000 lines. We incorporated nearly all patterns embodied in JHotDraw.
We did not implement the connection of figures,which uses the Strategy
pattern. We implemented painters, however,with the Strategy pattern. The
omitted pattern operates quite isolated fromother patterns. After imple-
menting the basic functionality we estimated that the effort necessary to

implement the connection functionality would be too high for the remain-
ing time. Fromstudying the patterns individually (see Chapter 3) we knew
that a GO implementations of this pattern is possible and feasible.

### 4.2 The Design of GoHotDraw

In this sectionwe describe the GoHotDrawframework. Like its Smalltalk
and Java counterparts, GoHotDraw is designed to create drawing editor
applications. These applications can be created by combining already exist-
ing features, as the framework can be easily extended. Newcomponents,
like tools or figures, need only to conformto the core interfaces and will fit
in seamlessly.
The GoHotDraw framework is designed according to theModel-View-
Controller (MVC) pattern. The MVC pattern is an architectural pattern.
The intent of the pattern is to separate the application into three distinct
units: the domain knowledge, the presentation, and the actions based on
user input. In the followingwe explain the three parts (Model, Viewand
Controller); describe themain types and interfaces; and highlight design
patterns used in the design of GoHotDraw.

38 CHAPTER 4. CASE STUDY: THE GOHOTDRAWFRAMEWORK
Figure 4.1 depicts the main interfaces and types of the GoHotDraw
framework.

![](data:image/png;base64,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)

Figure 4.1: Type diagramof GoHotDraw

4.2. THE DESIGN OF GOHOTDRAW 39

### 4.2.1 Model

“AModel is an active representation of an abstraction in the formof data
in a computing system” [80]. A model is a single object or a structure of
objects and “represents knowledge” [79]. Themodel is the domain-specific
representation of the data uponwhich the application operates.
TheModel of GoHotDraw represents the data users operate on. GoHot-
Draw’smodel comprises figures and their handles. Figures are elements
users can drawandmanipulate. Usersmanipulate figures by dragging han-
dles (the little rectangles on the perimeter of the top left figure in Figure 4.2
are handles).

![](data:image/png;base64,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)

Figure 4.2: Handles on a figure
The diagramin Figure 4.3 shows the types that formtheModel of the
GoHotDraw framework. In this section we will explain the elements of the
Model in detail, howthey relate to each other andwewill highlight how
the elements relate to design patterns.

40 CHAPTER 4. CASE STUDY: THE GOHOTDRAWFRAMEWORK

![](data:image/png;base64,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gRoIQJoIo/np0EgqE5BNYigAaSWM5jOD7skm1QyADBBGC4K+ezJHAzwAmSSMFhCRlPRSlwDIPGG04IiktJ36A0C2CKMFSiSlddQZALJLGC1oBt2TIpUEgIgIozFg0D1NUCsAiJQwGif67jmUmgBADhBG40ckRekDkDOE0bgSSeNJiQOQY4TReBNJY8L4JABylTCKQfcFzZMGALlNGOVjBt0XGOUIQD4QRmlA332+U3YA5A9hlEaIpPlIeQGQb4RRmiSS5gXf9wUgbwmjpEAkzVkKBYA8J4ySMoPuc4oYCkBBEEZpIYPuI+fMA1BAhFFaS9999jnbABQcYZS2EUmzwFcjAChcwijpIJJmiFMKQKETRkkfkTSNnEYA4kEYJd0Mum8jMRSAOBFGyQyD7lvBuQIgfoRRMkzffbO0IgMQY8IoWSGSHpYTAkDsCaNkkUhax0kAgP2EUbIu5pE0tgcOAIcjjBKRGA66F0MBoAFhlEjFYdB9rAI3ALSQMEpuKMi++wI7HADIAGGUXFIwkbQADgEAskIYJffkdSTN090GgIgIo+SqvIukebSrAJAzEkn3TjKkbuAOLeKSBCBOtIySSclFUe9BvklURL0HAJBVRVHvAAAA8SWMkj0ndBiWSFQc+FdUPLznZ8aMm7hgd3Od0jtWPz9owKXtis6reHJDKzaa3LMzkago6XB1s7tU9++fK6ua/isAIF1005Ntx3+2b6fisKd656rVax/89r1L1hX/9j+GNvH6eVf929K3tx036OwhvTq0feuj/2zEnM3Vz29bMPSokrofdu97bKfiT15zdElRSNSWl3dt175L27cIADRBGCXbZi6dddHRpSGENc891n/EQysf/s5/TR00cv9PDuvNjbtCCHf/ZOJf92yfoV2atuyha7vXf/P33puboc0BAHV00xOZEy647p96d0wma6f8bFMIYcvrL1w26KudjhjaofOlQ0b/4Nfba0III4/58tT1H4YQbj52xBenV4YQ3l049/zTrykrHVrS/uLPD5kw9/dVIYQ9H76TSFS073LLgXfe+rtJiURF+Wm/qrfFkcd8ec7m6hDCsK7DRy3f3sS+1eumr968YszwsWWlw7qfOG76sy8lEhWde89sers1O98+8LKX7r+v/MjzJ7xTddgDBICYE0aJ0vBzjwohbJi/ZfcHK884596nf7n+pMFfOKtf8ZI5Tw07fcruZLhhwo2Du5SEEC6768a/Obfrno/Wf+HiB15Y/v6Zw7903umd3lyy5LrB3099czdMuPHksnYhhLHf+tr1qbezJvd+7Yxv/mjhH/Z26tG7dONto6alvsXqba8O+8dnN1Xtqd7xu8MeIADEnDBKlNr3KA0h1OzYs+rB6e9U7z3l9snPzb77mf956NY+ZTvWLpi8vuqKW688u9O+MHrRLV+5/swu1VuWff68M665bdri+ff+YvHM0qLER1tfTn1zV9x65Wc77AujV//9VaMP6Zf/q/IRdaOXep33ar2/2rHux4+v+7C0rN+Kdx997a05j17TMfUt1lStuWzqPZV/nHvJ/FmHPcDU3woACpLvjBKlD97+KITQ+aSOlU9sDCH875RvdJ3yyW9fXFUVji879PVH9hr94B1dpj8+7/xBD7z127W7a5NFRWloXezWu7zs4+eyHuX1v726dfnyEMLRn7vxxA7FIYS/mDgi/OiHKb5zScc+j399cCKEN3+6KZUDBIC4EUaJ0qxl20MIJ19RXvzyvjA4eNZ37ul/ZN1vjxp4VL3Xb13xn/2H/jC073H1mCGX3jRm4ti7tnwqix78n2Tt3hbtxvd//UjDAUyfvGnNvrctLj14sSSKG76y0e0Wlx6TOPgfKR0gAMSNbnoi8+pD3/3J+9XFR3SbfMZRx3+lewjh/xbVVlScWlFxauWzS2fPXryhwbPSymm/2JNMnjPr/kf//eabLu/5p70HU2BRu7IQQs2H697ZVRtCeGVGZdObblFr6tGnnRxC2PzGYxt273vzV2f8vO5XqW83xQMEgLhxMyTbbhkyrnNx2L1z68rV20III6dO7nNE0e6xtxx35/g/PD7h1I3n9tm7bt6i9Uf2Gjz5ByX1/rbzwH3hb/k9Myet6bPo0adrkyFZW1MbQlFpt6u6t39i0wenDLjp9J67l76xpbGtd9w/n+ikr89YO+mmscelNHFpl8/ccPmxc+ZsWDHghLGnHVuz9LX36n6V+nYHpHaAABA3WkbJtrUrVi9fvnpVZVXvAQPvmjn96fH9QgilnU5+46U7LxnUd/WLv5r/8rYvjRq18PW7y4oS9f524PiJ4y7sv2vNL+9/8MXP3Trp+vIjkrW77nxtewhhxgu3VQzsVv3u2spdPR5+7vLGtv4P3xxS3qlk8cNz523eleoeJ0oe+8191w3rl9xYufzd4rsfuXb/Dw/uW4rbTfEAASBuEsmk2WXIjEQiJBdFvRNpUFO15sm5q4vadbzmqnNDCJtemVp+9vxjTrp981sXp39jiYrgkgQgTnTTQzMSRSV3jP3uH3fVzph94Wndw/M/XhhCuOhfzox6vwCgEGgZJWMKpWU0hPD+Kwtuvv2Jhcsqd+4t7tX/xCvHfXXK+LMysiUtowDEjDBKxhRQGM0eYRSAmDGACQCAyGgZJWMShoq3iksSgDgxgIlM0k3fUomKqPcAALJKNz0AAJERRgEAiIwwCgBAZIRRAAAiI4wCABAZYRQAgMgIowAARMak92SMSe9bxyUJQJyY9J5MMul9S5n0HoCY0U0PAEBkhFEAACIjjAIAEBlhFACAyAijAABERhgFACAy5hklY8wz2jouSQDi5P8DAAD//x0O+DvlYuhKAAAAAElFTkSuQmCC)

Figure 4.3: Model of GoHotDraw
Figure
Figure is a central abstraction in the GoHotDraw framework. Figure is
the common interface other GoHotDrawcomponentsworkwith. Figures
represent graphical figures that users can arrange to drawings. Figures can
drawthemselves and have handles for theirmanipulation.
DefaultFigure implements functionality common to all figures, like
handling of event listeners. Event listening is part of the Observer pattern,
that is used to update the View. Concrete implementations of Figure embed
DefaultFigure. All types embedding DefaultFigure can be observed. That
way it is easy to extend the selection of available figure types without
having to change the rest of the framework. Even though Figure hasmore
than 20methods, the implementation of RectangleFigure consists of less
than 60 lines of code.
The Figure type-hierarchy uses the TemplateMethod pattern frequently.

Template methods are defined in the DefaultFigure type. The concrete
implementations of Figure embed DefaultFigure and implement the hook
methods.

4.2. THE DESIGN OF GOHOTDRAW 41
Composite Figure and Drawing
CompositeFigure is a figure that is composed of several figures, which
in turn can be composite figures. With CompositeFigure, a composition
of figures can be treated like a single figure. CompositeFigure embeds
DefaultFigure andmaintains a collection of figure objects.
A Drawing is a container for figures. Drawings are displayed by views.
StandardDrawing implements the Drawing interface and the Figure inter-
face by virtue of embedding CompositeFigure.
The Composite pattern is used to enable us to treat simple figures (like
rectangles) and complex figures (like drawings) uniformly. The Figure inter-
face plays the role of Component, CompositeFigure and StandardDrawing
are the Composites, and RectangleFigure is a Leaf. The relationship between
these types is depicted on the left hand side of Figure 4.3.
Border Decorator

InGoHotDrawfigures are drawnwithout borders. To drawborders around
figures the figure objects get decorated with BorderDecorators. We imple-
mented a DefaultFigureDecorator, which implements the interface Figure.
DefaultFigureDecorator embeds DefaultFigure andmaintains a reference
to the figure object that is to be decorated. Method calls are forwarded to
the figure object.
The listing belowshows the definition of DefaultFigureDecorator and
one of itsmethods forwarding calls to the decorated figure. DefaultFigure-
Decorator acts as the base type for other decorators.
type DefaultFigureDecorator struct {
*DefaultFigure
figure Figure //the decorated figure
}
func (this *DefaultFigureDecorator) GetDisplayBox() *Rectangle {
return this.figure.GetDisplayBox()
}

42 CHAPTER 4. CASE STUDY: THE GOHOTDRAWFRAMEWORK
BorderDecorator embeds DefaultFigureDecorator, gaining its function-
ality. BorderDecorator overrides the Drawmethod, which draws the figure
and then a border on top:
func (this *BorderDecorator) Draw(g Graphics) {
this.DefaultFigureDecorator.Draw(g)
g.SetFGColor(Black)
g.DrawBorderFromRect(this.GetDisplayBox())
}
Clone has to be overridden to return a newinstance of BorderDecorator,
instead of an instance of the decorated figure.
func (this *BorderDecorator) Clone() Figure {
return NewBorderDecorator(this.figure)
}
The actual figure object is oblivious that it is decoratedwith a border and
other parts of the framework don’t have to distinguish between bordered
or un-bordered Figure objects, since DefaultFigureDecorator implements
the Figure interface.
Handle types

A Handle (displayed by a little rectangle, see Figure 4.2) is used to change
a figure by directmanipulation. A figure has eight handles (one on each
corner and one on each side). A handle knows its owning figure and
provides its location to track changes.
Handles use Locators to find the right position on a figure to display
the handles. Locators encapsulate a Strategy to locate a handle on a figure.
The handle is the Context for Locator objects.
For the creation ofHandles, the FactoryMethod pattern is used. Con-
crete implementations of Figure create handle objects on demand by im-
plementing GetHandles. In the listing below GetHandle creates a set
of handles for the current figure object. The actual factorymethod is the
function AddAllHandles. AddAllHandles first adds four handles (one
for each corner) to handles and then adds four handles for each side. The

4.2. THE DESIGN OF GOHOTDRAW 43
parameter figure gets passed to the handle creation, so that each handle
knows its figure. Note that handles is an output parameter.
func (this *RectangleFigure) GetHandles() *Set {
handles := NewSet()
AddAllHandles(this, handles)
return handles
}
func AddAllHandles(figure Figure, handles *Set) {
AddCornerHandles(f, handles)
handles.Push(newSouthHandle(f))
handles.Push(newEastHandle(f))
handles.Push(newNorthHandle(f))
handles.Push(newWestHandle(f))
}
These are simplifications of the Factory Method pattern. The listed
examples create objects and return them. Fully fledged factory methods
return different products depending on the dynamic type of their receiver.

### 4.2.2 View

“A view is a (visual) representation of itsmodel” [79] and is “. . . capable of
showing one or more [. . . ] representations of theModel” [80]. The view
renders themodel into a formsuitable for interaction.
The Viewpart of GoHotDrawis concernedwith displaying drawings.
The View does notmanipulate theModel directly. User input is forwarded
to the Controller. The diagrambelow show the type that are part of GoHot-
Draw’s View.

44 CHAPTER 4. CASE STUDY: THE GOHOTDRAWFRAMEWORK

![](data:image/png;base64,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)

Figure 4.4: Viewtypes of GoHotDraw
View Interface
View is another important interface in GoHotDraw. A view renders a
drawing and listens to its changes. It receives user input from the user
interface (Graphics) and forwards the mouse or key events to an editor.
Viewsmanage figure selections, including drawing of handles.
Drawing changes are propagated to viewswith the Observer pattern.
Drawings send FigureEvents to their listeners (the EventHandlers of views)
and the views redrawthemselves. The EventHandler type acts as listener
for views. The EventHandler receives events (figure added, removed,
changed) from the drawing and informs the view to update itself. Con-
trollers changing figures, trigger FigureEvent,which are send to the figures
listeners.
Graphics
Graphics is an abstraction of the user interface. The displaying and handing

of user input could be done with a variety of GUI-libraries. We used the
XGB library to communicate with the XWindow Server for user input and
graphical output.

4.2. THE DESIGN OF GOHOTDRAW 45
The XWindowSystem, or X11, is a software systemand network pro-
tocol that provides a graphical user interface (GUI) for any computer that
implements the X protocol. XBG provides low level functionality for com-
municationwith an X server. XGB is a GO port of the XCB library,written
in C. XGB is used to display the user interface and to capture user input.
We use the Adapter pattern to adapt the third-party graphic library
XGB, toGoHotDraw’sGraphics interface. We implement a XGBGraphics to
conformto the Graphics interface, thus enabling us to use the XGB library
with GoHotDraw.
Painter
A painter encapsulates an algorithm to render drawings in views. In
GoHotDraw, different update strategies can be defined and views select
the appropriate ones. We implemented a SimpleUpdateStrategy, which

redraws the entire drawing, butmore sophisticated strategies are possible,
like accumulating damage and only redrawing the parts affected. Viewis
the Context, Painter the Strategy interface, and SimpleUpdateStrategy a
Concrete Strategy.

### 4.2.3 Controller

“A controller is the link between a user and the system” [79]. The controller
receives input and initiates a response bymaking calls onmodel objects. A
controller accepts input fromthe user and instructs themodel and viewport
to performactions based on that input.
TheController part ofGoHotDrawis concernedwith figure creation and
manipulation. Clients don’t interactwith theModel (figures in a drawing)
directly. The Viewforwards user input to the Controllerwhichmanipulates
theModel accordingly. The diagrambelow shows the types that are part of
GoHotDraw’s Controller.

46 CHAPTER 4. CASE STUDY: THE GOHOTDRAWFRAMEWORK

![](data:image/png;base64,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)

Figure 4.5: Controller types of GoHotDraw
Editor
An editor coordinates and decouples views and tools. Editors maintain
the available and selected tools and delegate requests fromthe viewto the
currently selected tool.
The editor keeps references to its viewand its current tool. Views and
tools don’t communicate directlywith each other, but through the editor.
This is an example of theMediator pattern. Editors aremediators, views
and tools are colleagues.
The editor’s behaviour is dependent on its current tool. Clients define
which tool is the active tool. Depending on the dynamic type of the tool
object, the editor behaves differently. This is an example of the State pattern
with the editor being the context and the tools being the states.
Tool
Tool is another major interface in GoHotDraw. Tools define a mode of a
drawing view(create newfigures; select,move, resize figures).

InGoHotDrawwe useNullObject in the tools type hierarchy. NullTools
do nothing. Editors can be initialized with NullTools, so that should clients

4.2. THE DESIGN OF GOHOTDRAW 47
forget to set a specific tool, the editor will not cause a nil-pointer error, it
will just do nothing.
Creation Tool
A creation tool is a tool that is used to create new figures. We use the
Prototype pattern in creation tool. The figure to be created is specified by a
prototype. A creation tool creates newfigures by cloning the prototype.
The CreationTool maintains a reference to a prototypical instance of
Figure:
type CreationTool struct {
...
prototype Figure
}
On request, the prototype figure is cloned and returned:
func (this *CreationTool) createFigure() Figure {
if this.prototype == nil {
panic("No prototype defined")
}
return this.prototype.Clone()
}
All concrete figures have to implement the Clonemethod. The GoHot-
Drawspecific participants of the pattern are Figure as the Prototype and

creation tool as the client. The next listing shows the Clone methods of
RectangleFigure. RectangleFigure creates a new instance and copies the
prototype’s display box.
func (this *RectangleFigure) Clone() Figure {
figure := NewRectangleFigure()
figure.displayBox = this.displayBox
return figure
}
Selection Tool and Trackers
Aselection tool is a tool that is used to select andmanipulate figures. The be-
haviour of SelectionTool is implemented with the State pattern. Depending

48 CHAPTER 4. CASE STUDY: THE GOHOTDRAWFRAMEWORK
on external conditions, the SelectionTool’s current tool changes. A selection
tool can be in one of three states: figure selectionwith AreaTrackers, figure
movementwithDragTrackers, and handlemanipulationwithHandleTrack-
ers. An AreaTracker is a selection tool for background selection (selecting
one ormore figures). Onmouse drag events the area tracker informs the
view about figures contained in the area covered, so that the found figure’s
handles can be drawn. A DragTracker is a selection tool used to select and
move figures under themouse pointer. Onmouse down the drag tracker
either informs the viewto select the figure under the pointer or deselects
all figures. Onmouse drag the drag trackermoves the selected figures. The
moved figures inform the view about being moved. A HandleTracker is

a selection tool to manipulate handles. The handles do the actual figure
manipulation.
The following listing shows the method MouseDown. MouseDown al-
ters the state (currentTool) of SelectionTool. If the MouseEvent
happened on a handle, currentTool becomes a HandleTracker; if
the MouseEvent happened on a figure, the currentTool becomes a
DragTracker; otherwise it becomes an AreaTracker. Depending on
the dynamic type of currentTool, the call of MouseDownwill behave dif-
ferently. The HandleTrackers, DragTrackers and AreaTrackers are created
on demand by factorymethods.
func (this *SelectionTool) MouseDown(e *MouseEvent) {
selectedHandle := this.editor.GetView().FindHandle(e.GetPoint())
if selectedHandle != nil {
this.currentTool = NewHandleTracker(this.editor, selectedHandle)
} else {
selectedFigure := this.editor.GetView().GetDrawing().FindFigure(e.GetPoint())
if selectedFigure != nil {

this.currentTool = NewDragTracker(this.editor, selectedFigure)
} else {
if !e.IsShiftDown() {
this.editor.GetView().ClearSelection()
}
this.currentTool = NewAreaTracker(this.editor)
}
}

4.3. COMPARISON OF GOHOTDRAWAND JHOTDRAW 49
this.currentTool.MouseDown(e)
}

### 4.3 Comparison of GoHotDraw and JHotDraw

GoHotDraw’s design is very similar to the SmalltalkHotDrawand the Java
JHotDraw. GoHotDraw applies the same patterns for similar functionality
and theywork aswell in GO as in other languages.
As a result, GoHotDraw’s structs and embedding generally parallel
JHotDraw’s classes and inheritance, and GoHotDraw’s interfaces are gen-
erally similar to those in the Java versions.
JHotDraw is meant to be used to create drawing editor applications.
Its architecture allows for easy extension of the existing functionality and
for a convenient combination of existing components. We tried to achieve
the same with GoHotDraw. We used the same core abstractions (Figure,
View, Drawing, Editor, Tool) and provided common functionality. New
functionality that conform to those abstractions will integrate well and
allow for an convenient extension of the framework. We implemented a

drawing application, allowingmouse controlled figuremanipulation and
keyboard controlled tool selection, by using the functionality provided by
GoHotDraw,with only 90 lines of code.

### 4.3.1 Client-Specified Self in the Figure Interface

The key difference between the GO and the Java design comes fromthe dif-
ference between GO’s embedding and Java’s inheritance (see Section 2.1.2):
in GO methods on “inner” embedded types cannot call back tomethods
on “outer” embedding types. In contrast, Java super classmethodsmost
certainly can call “down” tomethods defined in their subclasses, and this is
the key to the TemplateMethod pattern. As a result, GoHotDraw’s Figure
interface is significantly different to the Java version. While both interfaces

50 CHAPTER 4. CASE STUDY: THE GOHOTDRAWFRAMEWORK
provide the samemethods,many (if notmost) of thosemethods have to
have an additional Figure parameter. Thosemethods are templatemethods
(see Section 3.5.1) and need to use the Client-Specified Self pattern (see
Section 2.2.2).
A simple example: A Figure is empty if its size is smaller than 3-by-
3 pixels. The method IsEmpty in the following listing is defined for
DefaultFigure (to be embedded, contains common functionality of all
Figure sub-types), and call the GetSize method defined in the Figure
interface.
func (this *DefaultFigure) IsEmpty(figure Figure) bool {
dimension := figure.GetSize(figure)
return dimension.Width < 3 || dimension.Height < 3
}
The problemis that this needs to call the GetSizemethod of the correct
sub-type: here RectangleFigure or CompositeFigure. Both types embed De-

faultFigure and provide a suitable definition for GetSize (as well as other
methods). In Java, DefaultFigure could simply call this.GetSize(), the
call will be dynamically dispatched and will run the correct method. In
GO, this callwould try to invoke the (non-existent) GetSizemethod on
DefaultFigure: a client-specified self is needed for dynamic dispatch.
This problem is exacerbated when a design needs multiple levels of
embedding or inheritance. Following JHotDraw, GoHotDraw’s DefaultFig-
ure is embedded in CompositeFigure; CompositeFigure is embedded in
Drawing; Drawing is then further embedded in StandardDrawing. These
multiple embeddingsmeanmany Figuremethods require a client-specified
self parameter towork correctly. Of 21methods in that interface, six require
the additional client-specified self argument, as the final version of the
Figure interface illustrates:

4.3. COMPARISON OF GOHOTDRAWAND JHOTDRAW 51
type Figure interface {
MoveBy(figure Figure, dx int, dy int)
basicMoveBy(dx int, dy int)
changed(figure Figure)
GetDisplayBox() *Rectangle
GetSize(figure Figure) *Dimension
IsEmpty(figure Figure) bool
Includes(figure Figure) bool
Draw(g Graphics)
GetHandles() *Set
GetFigures() *Set
SetDisplayBoxRect(figure Figure, rect *Rectangle)
SetDisplayBox(figure Figure, topLeft, bottomRight *Point)
setBasicDisplayBox(topLeft, bottomRight *Point)
GetListeners() *Set
AddFigureListener(l FigureListener)
RemoveFigureListener(l FigureListener)
Release()
GetZValue() int
SetZValue(zValue int)
Clone() Figure
Contains(point *Point) bool
}
The size of interfaces in GoHotDrawis bigger than the GO norm(“In
Go, interfaces are usually small: one or two or even zeromethods.” [71]).
Some of the interfaces in JHotDraw contain dozens ofmethods. We do not

believe that the size of an interface is a sign of good programdesign. We
could have implemented some of Figure’smethods as functions, reducing
the number ofmethods in its interface, but we doubt that this wouldmake
GoHotDraweasier to understand,maintain or extend.

### 4.3.2 GoHotDraw User Interface

The Java user interface and graphics framework Swing is tightly integrated
into JHotDraw. GO is designed to be a systems language. The UI capabili-
ties are rather limited and are stillmarked as experimental.

52 CHAPTER 4. CASE STUDY: THE GOHOTDRAWFRAMEWORK

![](data:image/png;base64,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6CwAyKKzACCLzgKALDoLALLoLADIorMAIIvOAoAsOgsAsugsAMiiswAgi84CgCw6CwCy6CwAyKKzACCLzgKALDoLALLoLADIorMAIIvOAoAsOgsAsugsAMiiswAgi84CgCw6CwCy6CwAyKKzACCLzgKALDoLALLoLADIorMAIIvOAoAsOgsAsugsAMiyhHoCCA1N04I/dblcgY8bGxtDMaNZFXx/xwiHuz/+EQg+HnRdD8WMrmZ0NnxVVlaOv7GsrCwUcwmBML/7PAKziXUDAJBFZwFAFusGGGuKtctwEOZ3HxLoLMYKh5dBxrwMGCwc7v7UjwCuONYNAEAWnQUAWawbhK8wv4InzO8+j8BsorNhKkxWIScT5nefR2CWsW4AALLoLADIorMAIIvOAoAsOgsAsugsAMiiswAgi84CgCw6CwCy6CwAyKKzACCLzgKALDoLALLoLADIorMAIIvOAoAsOgsAsugsAMiiswAgi84CgCw6CwCy6CwAyKKzACCLzgKALDoLALLoLADIorMAIIvOAoAsOgsAsugsAMiiswAgi84CgCw6CwCy6CwAyKKzACCLzgKALDoLALLoLADIorMAIIvOAoAsOgsAsugsAMiiswAgi84CgCw6CwCy6CwAyKKzACCLzgKALDoLALLoLADIorMAIIvOAoAsOgsAsugsAMiiswAgi84CgCw6CwCy6CwAyKKzACCLzgKALDoLALLoLADIorMAIIvOAoAsOgsAsugsAMiiswAgi84CgCw6CwCy6CwAyKKzACCLzgKALDoLALLoLADIorMAIIvOAoAsOgsAsugsAMiiswAgi84CgCw6CwCy6CwAyKKzACCLzgKALDoLALLoLADIorMAIMsS/El7W2voZgIAVyfOZwFA1sj57LmzDaGeCQBcnbStd94e6jkAwNXs/wIAAP//OWXbD+J4PXoAAAAASUVORK5CYII=)

Figure 4.6: User interface GoHotDraw
Figure 4.6 shows a screenshot of a sample application created with
GoHotDraw. Tools can be selected with function keys and the mouse is
used for rectangle creation andmanipulation.
Graphics library
GO is a systems systems language. The UI libraries are not very mature
yet. We failed to get GO’s standard graphics library “exp.draw.x11” (exp
for experimental) towork on ourmachine. We tried the go-gtk framework
[62]. We compiled the library and a simple application, but it crashed
with a segment fault error. Eventually we settled for the “XGB framework
Go-language Binding for the XWindowSystem”.
The XWindowSystem, or X11, is a software systemand network pro-
tocol that provides a graphical user interface (GUI) for any computer that
implements the X protocol. XGB is a GO port of the XCB library, written

in C. XBG provides lowlevel functionality for communicationwith an X
server. The XGB project’s activity seized. Updates and changes of the GO

4.3. COMPARISON OF GOHOTDRAWAND JHOTDRAW 53
language were not incorporated for some months. We continued to use
XGBwith an older version of GO.
To limit the impact a change in the graphics library would have, we
designed an additional adaptation layer. GoHotDraw uses the Graphics
interface to showawindowand render the shapes. XGBGraphicsmaintains
a reference to an instance of xgb.Conn (a connection is a link between
clients and the X-Server) as depicted in the diagrambelow:

![](data:image/png;base64,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)

Figure 4.7: XGBGraphics adapting XGB library
The following listing shows one of the adaptermethods. DrawBorder
takes the location and dimension of a rectangle, creates a XGB-conform
rectangle and sends the rectangle via the connection to the X-server to draw
the border of a rectangle.
func (this *XGBGraphics) DrawBorder(x, y, width, height int) {
rect := this.createRectangle(x, y, width, height)
this.connection.PolyRectangle(this.pixmapId, this.contextId, rect)
}
In terms of the Adapter pattern, the interface Graphics is the Target,
XGBGraphics the Adapter, xgb.Conn (the xgb connection to the X-Server)
is the Adaptee, and GoHotDraw is the Client. XGBGraphics also shields

54 CHAPTER 4. CASE STUDY: THE GOHOTDRAWFRAMEWORK
clients from having to fully understand the workings of the XGB library
(connections, pixmaps, contexts...),making XGBGraphics a Fac¸ade for the
XGB framework.
The current GUI library could be replaced by adapting the new library
to the Graphics interface. This layer of abstraction is unique to GoHot-
Draw and is not present in JHotDraw. JHotDraw initially used the AWT
framework andmajor have been necessary in to switching to Swing. Even
though we have the adaptation layer in place we did not replace XGB with
a different library.
Low-level Functionality
Apart fromdisplay purposes the JHotDraw uses the Swing framework for
calculations regarding points, rectangles and dimensions. To be graphic
library independent,we implemented our own Dimension, Point, and Rect-
angle types. The following listing shows the type Rectangle. A rectangle is

represented by the x and y coordinates of their top left corner and itswidth
and height. We declared the Rectangle’smembers public for convenience
to not having to declare and use separate getters and setter.
The functionality for handling rectangles – grow, translate, union, con-
tains – wasmainly ported fromJava’s Swing framework. The porting was
straight forward, but inconvenient due to the necessity of type casts for
numbers. Our Rectangle type, aswell as Swing’s,workwith integers. The
position of a rectangle can be negative if its top left corner is outside the
top or left of the canvas. We allow negative sizes to avoid type errors at
runtime. Negative sizes are interpreted as zero. We used GO’smath library,
which works with floating point numbers. The type differencesmake type
casts necessary. Themethod ContainsRect in the listing below checks if

the passed in rectangle rect is fully contained in the receiver object this.
Themethod uses themath library – part of the standard GO distribution.
The Fmin and Fmax functions of themath package take float64 numbers
as arguments and return theminimumormaximumas float64, therefore,

4.3. COMPARISON OF GOHOTDRAWAND JHOTDRAW 55
the result has to be cast back to int.
func (this *Rectangle) ContainsRect(rect *Rectangle) bool {
return (
rect.X >= this.X && rect.Y >= this.Y &&
(rect.X+int(math.Fmax(0, float64(rect.Width)))) <=
this.X+int(math.Fmax(0, float64(this.Width))) &&
(rect.Y+int(math.Fmax(0, float64(rect.Height)))) <=
this.Y+int(math.Fmax(0, float64(this.Height))))
}
GO’s strict type system make type cast necessary. This is not only
annoying to program, butmakes it harder to read andmaintain.

### 4.3.3 Event Handling

Transforming user input into events is a little bit awkward. The graphics
library XGB can be polled for events. The events returned are of type
xgb.Event, which is an interface. To extractwhat kind of event happened, a
type switch is necessary.
Listing 4.1 shows an example for the ButtonPressEvent (akaMouse-
Down). The StartListeningmethod repeatedly polls for event replies
from the X-server. The returned event is of interface type xgb.Event.
xgb.Event is an empty interface. A type switch determines the dy-
namic type of the reply object. If the reply is a ButtonPressEvent,
a new MouseEvent gets instantiated; the event’s X and Y coordinates,
the pressed button, and the keymodifier (pressed shift key for example)
are extracted fromxgbEvent. The created event is then forwarded to the
fireMouseDownmethod. fireMouseDown calls MouseDown on all of its

listeners and forwards the MouseEvent object. In the current implementa-
tion of GoHotDraw, the listeners are applications. Applicationsmaintain a
reference to the graphics object and to an editor. The editor in turn knows
its tools and view.
The troublesome part about this kind of implementation is that with
each newkind of input event, the switch-case has to be extended. This in
not only tedious, but also hard tomaintain, and bad object-oriented style.

56 CHAPTER 4. CASE STUDY: THE GOHOTDRAWFRAMEWORK
func (this *XGBGraphics) StartListening() {
for {
reply := this.GetEventReply()
switch xgbEvent := reply.(type) {
case xgb.ButtonPressEvent:
event := &MouseEvent{}
event.X = int(xgbEvent.EventX)
event.Y = int(xgbEvent.EventY)
event.Button = int(xgbEvent.Detail)
event.KeyModifier = int(xgbEvent.State)
this.fireMouseDown(event)
}
//the other events (MouseUp, MouseDrag etc.)
}
}
func (this *XGBGraphics) fireMouseDown(event *MouseEvent) {
for i := 0; i < this.listeners.Len(); i++ {
currentListener := this.listeners.At(i).(InputListener)
currentListener.MouseDown(event)
}
}
Listing 4.1: Transforming XGB events into GoHotDrawevents
We simplified JHotDraw’s event handling. The simplificationwas not
due to GO specifics but a deliberate design decision, currentlywe support
only one drawing and one view active at a time. We did not implement

separate drawing events. The Drawing sends figure events to the view
aswell. An extension to allowmultiple views of drawingsmightmake a
DrawingEvent type necessary.

### 4.3.4 Collections

JHotDrawmaintains references to other objects in a variety of collections
(ArrayList, Vector, Enumeration). GO’s collection library is still very lim-
ited. We started out using Vector, but realized soon that a Set type was
required. The implementation of Set was easy enough sincemost of Vec-
tor’s functionalitywas reused.

4.3. COMPARISON OF GOHOTDRAWAND JHOTDRAW 57
Our Set embedded Vector andwe overrode the Pushmethod.
type Set struct {
*vector.Vector
}
func (this *Set) Push(element interface{}) {
this.Add(element)
}
func (this *Set) Add(element interface{}) {
if !this.Contains(element) {
this.Vector.Push(element)
}
}
We also implemented other convenience methods, like Contains, Re-
place, and a Removemethod based on object identity rather than indexes:
func (this *Set) Remove(element interface{}) {
for i := 0; i < this.Vector.Len(); i++ {
currentElement := this.Vector.At(i)
if currentElement == element {
this.Vector.Delete(i)
return
}
}
}

58 CHAPTER 4. CASE STUDY: THE GOHOTDRAWFRAMEWORK

# Chapter 5

# Evaluation

In this Chapterwe reflect onwhatwe have learned about GO fromimple-
menting design patterns and GoHotDraw. We discuss GO functionality,
GO idiomswe found, and tools providedwith GO.

### 5.1 Client-Specified Self

A common design principle of frameworks is Inversion of Control [33]:
frameworks provide interfaces that clients implement; the framework then
calls the client’s code, instead of the client calling the framework. This is
also known as theHollywood Principle: “don’t call us,we call you” [12].
Inversion of Control is often achieved by applying the TemplateMethod
pattern: the framework defines the algorithmand provides hookmethods
that are to be implemented in client code. In GO, TemplateMethod (and
with that frameworks)will have to apply the Client-Specified Self pattern
with all of its inconveniences. We are not aware of frameworkswritten in
GO, butwe expect to see Client-Specified Self being appliedwidely, causing
suboptimal designs.
Using client-specified self is less satisfactory than inheritance: it re-
quires collaboration of an object’s client to work; the invariant that the

self parameter is in fact the self is not enforced by the compiler, and
59

60 CHAPTER 5. EVALUATION
there is scope for error if an object other than the correct one is passed
in (e.g. chess.Play(monopoly) see Appendix A.3.10). Furthermore,
client-specified self is a cause for confusion on the implementer side aswell:
are hookmethods to be called on the templatemethod’s receiver, or on the
the passed in parameter? The extra parameter complicates code,making it
harder to read andwrite, for no clear benefit over inheritance.

### 5.2 Polymorphic Type Hierarchies

Inmost object-oriented languages types can formpolymorphic type hierar-
chies. A type can be extended by adding and overriding existing behaviour.
The extended type is a subtype of the original, in that any value of the
extended type can be regarded as a value of the original type by ignoring
the additional fields.
In GO, types can only be used polymorphically if there is an interface
defined that all subtypes implement. The supertype can either implement
all or only some of the interface’smethods.
It is considered good object-oriented design to define the supertype in a
type hierarchy as an abstract class [19]. Abstract classes are types which
are partially implemented and cannot be instantiated. Abstract classes are
commonly used both in design patterns and object-oriented programming
in general. Abstract classes define an interface and provide implementa-

tions of common functionality that subtypes can either use or override.
Interfaces can be used to define methods without implementations, but
these cannot easily be combined with partial implementations. GO does
not have an equivalent language construct to abstract classes.
GO’s implicit interface declarations provide a partial work around:
provide an interface listing methods, and a type that provides default
behaviour of all or a subset of the methods. In Java, classes implement-
ing the interface would extend the abstract class and define the missing
methods. In GO, types implementing an interfacewould embed the type

5.3. EMBEDDING 61
that is providing default implementations and implement the missing
functionality. In our designs, we applied a naming convention for types
providing default behaviour for an interface. We suffix the interface name
with “default” (consider the interface Foo, the type to be embedded is then
called DefaultFoo, and a concrete type embeds it: type ConcreteFoo
struct {DefaultFoo}).
The methods the default type are not implementing are effectively
abstractmethods (C++’s pure virtual or Smalltalk’s subclassResponsibility),
since the compilerwill raise an error if the embedding type does not provide
themissingmethods. We consider this less convenient than abstract classes,
because it involves two language constructs: the interface and the type
implementing default behaviour. Furthermore, the default type has to be
embedded and potentially initialized (see Section 5.3).

Another property of abstract classes is that they cannot be instantiated.
Most default types we implemented were meant to be embedded only
and not to be instantiated directly. There is no feature in GO to allow
embedding and prevent instantiation. Types can always be instantiated
within packages. Defining a type’s visibility to package privatemitigates
that problem, as only publicly defined functions of that package can return
an instance of the private type. But there is amajor catch here,making a
type package private prevents clients frombeing able to embed the type.
These are contradicting forces. Either instantiation can be controlled and
the type can’t be embedded, or embedding is possible but the type can be
instantiated freely.

### 5.3 Embedding

GO favours composition over inheritance. The Gang-of-Four’s experience
was that designers overused inheritance in their principle “favor object
composition over class inheritance” [39, p.20]. Since embedding is an
automated form of composition, it is not obvious whether the same will

62 CHAPTER 5. EVALUATION
apply to embedding vis-a`-vis composition. Embedding hasmany of the
drawbacks of inheritance: it affects the public interface of objects, it is not
fine-grained (i.e, no method-level control over embedding), methods of
embedded objects cannot be hidden, and it is static. Embedding should
be used with caution, since the entire public interface of the embedded
type is published to clients of the embedding type (fields and methods).
The alternative is to use compositionwith delegation. There is nomiddle
ground in GO to selectively publish certainmethods and fields.

### 5.3.1 Initialization

An embedded type is actually an object providing its functionality (meth-
ods andmembers). If any of the embedded objects is of pointer type the
embedded object needs to be instantiated. Not initializing the embedded
type can result in runtime errors. We see this as a disadvantage compared
with class based inheritance. Having to initialize embedded types is un-
wieldy and easy to forget, because GO does not have constructorswhich
could conveniently be used to initialize an object.

### 5.3.2 Multiple Embedding

We foundmultiple embedding helpful in implementing the Observer pat-
tern. A public type had to be defined providing the observable behaviour
(add, remove and notify observers). Every type could bemade observable
by embedding that type. The embedding type could still override the
embedded type’smethods and provide different behaviour if necessary.
As withmultiple inheritance,multiple embedding introduces the prob-
lemof ambiguousmembers. We find GO solves this problemin a nice way:
ambiguity is not an error, only calling an ambiguousmember is amistake
and ambiguous calls are detected at compile time. The ambiguity can be
resolved by fully qualifying themember in question.

5.4. INTERFACES AND STRUCTURAL SUBTYPING 63

### 5.4 Interfaces and Structural Subtyping

We think GO’s approach to polymorphism, through interfaces and struc-
tural subtyping instead of class-based polymorphism, is a good one, be-
cause interface inference reduces syntactical syntactic overhead, and GO’s
concise and straightforward syntax for interfaces declaration encourages
the use of interfaces. Using interfaces reduces dependencies,which is ex-
pressed in the Gang-of-Four’s principle of “programto an interface, not an
implementation” [39, p.18].
GO uses nominal typing and structural subtyping: types and interfaces
have names, butwhether a type implements an interface depends on the
type’s structure. Parameters in GO are often given empty interface type
which indicate the kind of object expected by themethod, rather than any
expected functionality. This idiomis common in other languages, includ-

ing Java: e.g., Cloneable in the standard library. In GO, however, all
objects implicitly implement empty interfaces, so using an empty interface,
even a named one, does not help the compiler check the programmer’s
intent. With named empty interfaces one gets the documentary value of
the nominal typing, but loses the compiler checking of static typing.
An alternative solution is to use interfaceswith a single, or very small
number of,methods. This lessens the likelihood of objects implementing
the interface by accident—but does not remove it. We found this idiom
used in the standard library (“In GO, interfaces are usually small: one or
two or even zeromethods.” [71]) and used it frequently in our own code.
Unlike much GO code, however, the key interfaces in GoHotDraw have
several tens ofmethods, closer to design practice in other object-oriented
languages.

64 CHAPTER 5. EVALUATION

### 5.5 Object Creation

We found GO’smultifarious object creation syntax—some with and some
without the new keyword—hard to interpret. Having to remember when
to use new andwhen to use makemakeswriting codemore difficult than it
needs to be. Furthermore the lack of explicit constructors and a consistent
naming convention can require clients to refer to the documentation or to
jump back and forth between definition and use in order to find the right
function for object creation.
GO is a systems language and thus requires access to low-level func-
tionality. The built-in function new() accepts a type as parameter, creates
an instance of that type, and returns a pointer to the object. The function
new can be overridden, giving GO programmers a powerful tool for defin-
ing customobject-instantiation functionality. The possibility of overriding
applies to all built-in functions.

We think GO should have constructors. Many complex objects need to
be initialized on instantiation, in particular if pointer types are embedded.
Without function overloading it is hard to provide convenience functions
creating the same kind of objectwith alternative parameters. Object creation
would be simplifiedwith constructors andwith constructor overloading.
The lack of constructors could bemitigatedwith a consistent naming
convention for factory function;we tried to stick to such a convention. If
there is only one type in the package, the type is called the same name
as the package, the function is called New returning an object of that type
(e.g. package ‘‘container/list’’, list contains the type List,
list.New() returns a List object). In caseswithmultiple types per pack-
age the method is to be called New plus the type name it creates. The

convention fails if convenience functions accepting different parameters
are needed.

5.6. METHOD AND FUNCTION OVERLOADING 65

### 5.6 Method and Function Overloading

Methods with the same name but different signature (number and type of
input and output parameters) are called overloadedmethods. The value
of overloaded methods is that it allows related methods to be accessed
with a common name. Overloading allows programmers to indicate the
conceptual identity of differentmethods. The problemis that overloaded
methods do not have to be related.
The systemwe adopted to get around overloading is to add a descrip-
tion of the method’s parameters to the method name. It is easy to lapse
on that convention and different developers can have different ideas of
what a “description of the parameters” is. The necessary differentiation of
themethodswith differing namesmakes the names longer and harder to
remember. Even thoughwe designed thesemethodswe found ourselves
going back to their definition to find the right name.

On the other handmethods that are overloaded excessively can be hard
to be used. It is hard to discernwhichmethod is called by reading the code,
especially if the parameters of overloaded methods are of types that are
subtypes of other possible parameters.
Function overloadingwouldmake convenience functions and factory
functions accepting different parameters easier to declare. GO’s designers
claim that the benefit of overloading is not high enough to make GO’s
type systemandwith that the compilermore complex [5]. We don’t think
method and function overloading is a necessary feature, but we would like
to have constructor overloading (see Section 5.5).

### 5.7 Source Code Organization

Code in Java or C++ is structured in classeswith one public class per file.
Source code in GO is structured in packages. A package can spanmultiple
files. A file can containmore than one type, interface or function. Methods

66 CHAPTER 5. EVALUATION
don’t have to be declared in the same file as the type they belong to. This
requires discipline tomaintain a usable structure. The GO documentation
gives no indication about howsource code should be organized. Should
types and theirmethods be in one file or separated, each type in a separate
file, where do interfaces go? Developers are left to their own devices which
can make understanding third-party libraries hard. Defining methods
outside classesmakesmethods hard to find, and hard to distinguish from
functions in the same package. This freedom also opens up possibilities
however. Related functions can be grouped in their own source file rather
than being staticmethods of a certain type.

### 5.8 Syntax

The C-like syntax made the transition from Java to GO easy. Interfaces
and type declarations are concise. The optional line-end semicolons are
convenient and encourage one instruction per line, at the cost of having to
followrules regarding placement of braces.

### 5.8.1 Explicit Receiver Naming

We found having to name receivers explicitly tedious and error prone. A
default receiver like this or self could ease the problem by allowing
developers to name the receiver, but not forcing them to do so. Most
receiver names in the standard library seem to favour brevity, using the
first character of the receiver type (e.g. func (f Foo) M()). Wemostly
used this due to our Java background, but found reading existing code
harder, becausewewere unsurewhich argument named the receiver.

### 5.8.2 Built-in Data Structures

GO’s data structures do not provide uniformaccess to theirmembers. There
is no uniformway for iterating over elements of collections. for-loopswith

5.8. SYNTAX 67
a range clause work on maps, arrays/slices, strings and channels, and
returnmostly two parameters (index and retrieved element), but in case of
channels only the retrieved element—without an index. Furthermore there
is no convention or facility for iterating over client defined data-structures.
Arrays and Slices can be copied easily, but there is no built-in functionality
to copy amap.
The syntax for deleting elements frommaps is unsatisfactory:
aMap[key] = aValue, false. If the boolean on the right is false,
the element gets deleted. The object representing the entry to be deleted
(here aValue) has to be compatible with the values of themap. If aValue
is of pointer type nil can be used, but in all other cases aValue has to be
a value of the concrete type. If themap contains strings, the empty string

"" could be used, for numbers 0 is possible and so on. Built-in types in
GO do not havemethods enabling amore conventionalway of deleting an
element frommaps.

### 5.8.3 Member Visibility

Having only visibility scopes “package” and “public” takes the optimistic
view that packages only encapsulate a single responsibility. There is no
mechanismto prevent instantiation or access to package privatemembers
within a package. Encapsulation within packages cannot be guaranteed
as seen in the Singleton (Appendix A.1.5) andMemento (Appendix A.3.6)
pattern. We think that using the first letter of an identifier to declare
visibility is bad as it is too easily overlooked.

### 5.8.4 Multiple Return Values

The multiple return value facility is often used to return an error signal:
the first return value is the result and the second is used to signal an
error. This “Comma OK” idiom is encouraged by the GO authors [2] and
used in the libraries; unsurprisingly, it is hard to avoid. An advantage

68 CHAPTER 5. EVALUATION
of this idiom is that GO’s exception handling mechanism is rarely used
and programs are not littered with try...catch blocks, which harm
readability. Furthermore, values like nil, EOF or -1 do not have to be
misused as in-band error values. On the other hand, error conditions are
easier ignored or forgotten because error checking is not enforced by the
compiler.

### 5.8.5 Interface Values

Methods can be declared on value types and on pointer types. Variables of
interface type can hold values and pointers. This can help to abstract away
the need for a client to know if the value is of pointer type or not, but on
the other hand it is inconsistent with “normal” types. Fromlooking at the
source code it is not apparent if a variable is holding a pointer or a value.

# Chapter 6

# Conclusions

GO’s concise syntax, type inference in combinationwith types implicitly
implementing interfaces, first class functions and closures, and fast compi-
lation,made programming in GO very enjoyable.
GO’s interfaces are easy to declare. Types implement interfaces au-
tomatically. Combined with GO’s approach to polymorphism through
interfaces, GO encourages good object-oriented design, as expressed in the
Gang-of-Four’s principle of “programto an interface, not an implementa-
tion” [39, p.18]. On the other hand, GO’s unusual approach to dynamic
dispatch requires the application of the Client-Specified Self pattern which
is inconvenient andworsens designs.
GO lacks abstract classes. Abstract classes combine interface definitions
and common functionality. GO’s interface definitions are straightforward,
but having to define and embed a separate type to provide default be-

haviour is inconvenient and also allows errors.
We found that embedding is a poor replacement for inheritance. Embed-
ding a type publishes all of the embedded type’s publicmembers, because
GO has only two visibility scopes and GO does not provide an alternative
mechanismfor hidingmembers of embedded types. Having to initialize
the embedded type is inconvenient and easy to forget. The only alternative
to embedding is to use composition and to forward calls. This is an all
69

70 CHAPTER 6. CONCLUSIONS
or nothing approach, reducing the code reuse capability of embedding
drastically.
Most patterns and applications can be implemented in GO to a great
extent as in Java. Our implementations might not be the most idiomatic
solutions, but that can tell programming language designers something
too: how steep is the learning curve, how could that be improved, is better
documentation needed, should certain solutions be more obvious? We
hope this thesis helps to start and foster a discussion of ways to implement
design patterns in GO, to discover GO specific patterns and idioms, and to
help identifywhich features GO mightwant to be added or changed.
Rob Pike has described design patterns as “add-on models” for lan-
guages whose “standard model is oversold” [71]. We disagree: patterns
are a valuable tool for helping to design software that is easy tomaintain

and extend. GO’s language features have not replaced design patterns: we
found that only Adapter was significantly simpler in GO than Java, and
some patterns, such as TemplateMethod, aremore difficult. However, first
class functions and closures simplify the Strategy, State and Command
patterns. In general, we were surprised by how similar the GO pattern
implementations were to implementations in other languages such as C++
and Java.

### 6.1 RelatedWork

As far as we know, we are the first to use design patterns to evaluate
a programming language. Norvig analysed how features of the Dylan
programming language affects implementations of design patterns and
he concludes that 16 of the 23 design patterns disappear or are easier to
implement with this dynamically typed language [66]. Where Norvig
focusses on replacing and simplifying patterns, we focus on evaluating
GO’s language features and patterns are ameans to that evaluation. We did
not try to find the simplest pattern implementations, but those thatmight

6.2. FUTUREWORK 71
be used in everyday programs.
Agerbo and Cornils analysed how and which design patterns can be
supplantedwith language features [8, 9]. Agerbo and Cornils tookmultiple
languages into account to support their thesis, that design patterns should
be programming language independent. We focussed on a single program-
ming language, GO, and how GO’s features affect the implementation of
design patterns.
Many pattern catalogues presenting the patterns of Design Patterns
for various programming languages have been published (e.g. Java [65],
JavaScript [46], Ruby [67], Smalltalk [11], C] [15]). We add to this collec-
tion the appendix of this thesis: a design patterns catalogue for the GO
programming language, albeit focussing primarily on the implementation
of the solutions.
Empirical studies conduct experiments to evaluate and compare pro-

gramming languages. Prechelt [76, 77] and Gat [41] had a number of stu-
dents implement the same programin different languages and compared
the languageswith quantitativemetrics like development time,memory
usage, execution speed. In contrast, our study is qualitative and based on
case studies.

### 6.2 FutureWork

GO is a systems language with a focus on concurrency and networking. In
this thesiswe implemented general purpose patterns in GO. We’d like to
extend this study by implementing concurrency and networking patterns
[84] in GO to evaluate GO’s features in that area.
GO is a newprogramming language combining existing and newcon-
cepts. We have a strong Java background. A catalogue of GO idioms and
GO specific patterns would help to make learning GO and developing
GO idiomatic programs easier. In this study we implemented existing
patterns in GO. In a follow-up studywewill investigate GO applications

72 CHAPTER 6. CONCLUSIONS
and libraries to collect idioms and patterns that have emerged.
The interfaces in GoHotDraw are bigger than GO’s preferred two, one
or even none methods [71]. Refactoring GoHotDraw towards smaller
interfaces could discover valuable insights into the differences between GO
and Java programs.

### 6.3 Summary

In this thesis we have presented an evaluation of the GO programming
language; GoHotDraw, a GO port of the pattern dense drawing application
framework JHotDraw; and a catalogue of GO implementations of all 23
Design Patterns described in Design Patterns [39]. We have found thatmost
pattern implementations in GO are similar to implementations in Java, that
GO’s approach to dynamic dispatch on subtypes complicates designs, and
that embedding hasmajor drawbacks compared to inheritance.

# Appendix A

# Design Pattern Catalogue

In this Appendix we present our GO implementations of all 23 design
pattern fromDesign Patterns. We organise the patterns in the same order as
they appear inDesign Patterns. There are three sections: Creational Patterns,
Structural Patterns and Behavioural Patterns. Each pattern description
consists of Intent, Context, Examples and Discussion.
73

74 APPENDIX A. DESIGN PATTERN CATALOGUE

### A.1 Creational Patterns

Creational patterns dealwith object creationmechanisms, trying to create
objects in amanner suitable to the situation.

### A.1.1 Abstract Factory

Intent Insulate the creation of families of objects fromtheir usagewithout
specifying their concrete types.
Consider the creation of amazes for a fantasy game. Mazes can come in
a variety of types such as enchanted or bombedmazes. Amaze consists of
map sites like rooms,walls and doors. Instantiating the right type ofmap
sites throughout the applicationmakes it hard to change the type ofmaze.
A solution is a factory interface that defines methods to create an in-
stance of each interface that represents amap site.

![](data:image/png;base64,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JSYmquu9evX6/fff1XVvb++EhAR1vVu3bv/5z3/UdR8fn7i4OHW9Y8eOx48fV9erVasWExOjrrdp0+b06dPqeo0aNaKjo9X1Fi1anD9/Xl0PCAi4f/++ut6kSZMrV66o64GBgXfu3FHXn3766T/++EN5nRsMBvmHunXrRkZGqsc/9dRT5up//PGHuh4cHHz79m3N+9XMWbt2bXPLdeHCBXW9Vq1aDx8+VNebN29+5swZdd3f39/c83LixAl1vXr16uae9yNHjqjrvr6+5l5XBw4cUNd9fHySkpLU9V69eu3Zs0dd9/b2TklJUdf79+//888/q+uenp5paWnq+uDBg//5z3+q6x4eHunp6er68OHDN2/erK67ubllZmaq62+++ebatWvVdVdX16ysLHV94sSJK1asUNddXFyys7PV9dDQ0AULFqjrzs7OOTk56vqnn346a9Ysdd3T0zMjI0Nd37hx44gRI9R1Dw8PzeX95z//+fLLL6vrbm5umvl37drVp08fdd3V1VUz/4EDB7p27aquu7i4aF4o+fjx461bt1bXnZ2d8/Ly1PULFy6EhISo605OTvn5+er6zZs3g4KC1HVHR0fNnWZ0dLT66u/5+flOTk7qwZIkxcfHq8+On56ebu6U+cnJyW5ubuobqVGjhnqwwWBIS0tT3/XDhw81z8BgMBiysrLs7OwK1aOioho0aKAeb2dnp/mkX758+dlnn1XX7e3tNVeKU6dOaXaNHRwcNF+Ehw4d6tmzp7ru6Oio+SL/9ddfBw4cqK47OTlpbgS+//77YcOGqesuLi6aO/fDhw+/8MIL6rqvr6/mdJjff/9dcyWqUaPGzZs31fXdu3dr5gkICNDcGf3000+jRo1S12vXrq25M9q+fbvmGZ3q1q178eJFdX3Tpk3vvPOOul6/fv1z586p62vWrPnLX/6irjdq1CgiIkJdX7ly5ccff6yuN27cWPMgZMmSJdOnT1fXmzVrdvToUXV9/vz5c+fOVddbtmwZFhamrs+dO3f+/Pnqetu2bffv36+uT58+fcmSJep6x44d9+7dq65//PHHK1euVNefe+65Xbt2qeuhoaGrV69W13v27Kl5kDl58uT169er63369NHcOb799ttbtmxR1wcMGLB161Z1/a233tqxY4e6PmjQoI0bN6rrb7zxxo8//qiuDx06VHNnOnz48F9+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amoykgMA+B+5cBWAPPDEE09s3Ljx9ddfj10EAABgAhjJAZAHGhoaHnjggV27dsUuAgAAJB566KF77rmno6MjdpE8ZiQHAADAyXXBBRfcd999dXV1sYsAE+MXv/jFww8/3N3dHbtIHnMvOQAAAE6u/3dM7BYAOcQqOQAAAADIKiM5AAAAAMgqF65SECoqKo4cOfLJT37yrbfeit0FAPLVF77whba2tqqqqthFAADynpEcBWF0dDSRSAwNDcUuAgB57KGHHopdAQBgkjCSAyAPXHvttYsXL77oootiFwEAAJgARnIA5IG1x8RuAQAAfOT+++/v7u6ePXt27CJ5rCh1QR9MbuXl5clksra2trm5OXYXAAAoOK+//vq2bdsuuuiiz33uc7G7AOQET1wFAADg5Nq5c+fGjRu3bNkSuwhArjCSAwAAAICsMpKj0JWXlxeFfPrTnw7mp0yZEsyvWLEio/wVV1wRzJeWlgbzq1evDuZLSkqC+Q0bNmSU/9KXvhTMFxcXB/M333xzRvnbbrsto/y9994bzAfDRUVFmzdvzij/ox/9KKP8448/nlH+t7/9bUb5bdu2ZZR/+eWXM8o3NTVllG9ra8son0wmx4YHBgbS5YP3TGhtbU2XD5Z5//33M8q/8cYbGeVffPHFYLi4OHzo3L59e0b5J598MqP8r3/962C+pKQkmH/88cczyj/yyCPBfGlp+LazDz74YDA/ZcqUYH7jxo3BfFlZWTB/1113BfPl5eXB/G233RbMV1RUBPO33HJLMD9t2rRg/qabbgrmZ8yYEcxfe+21wXxlZWUwv27dumC+qqoqmF+9enUwn+5+LldccUUwP2fOnGB+xYoVwfypp54azH/5y18O5pcvXx7MX3/99cH8JZdcEsxfc801wfxnP/vZYP7qq68O5letWhXM19XVBfPpbmG5cuXKYD7dwTfdz/O6664L5pcvXx7M33TTTcH8hRdeGMynO1iff/75wfzXvva1YH7JkiXB/O233x7ML168OJi/++67g/mzzz47mP/Wt74VzJ955pnB/KZNm4L5008/PZhP9wThmpqaYP7HP/5xMD9v3rxg/rHHHgvmq6urg/lf/epXwfysWbOC+d/85jfB/MyZM4P5J554IpifMWNGML9169Zgftq0acH8M888E8xXVFSkArfeemsikfjlL38545iWlpZgfsmSJTNC9u3bF8zX1tYG8+3t7cH8okWLgvmOjo5g/vTTTw/me3p6gvlTTz01mB8YGAjmZ8+eHcwPDQ0F85WVlWPD6Q4uQ0NDwW+e7mAxMDAQzKfb+ff09ATzp59+ejDf0dERzC9cuDCYb29vD+Zra2uD+X379gXzS5YsCeZbWlqC+aVLlwbzzc3NwXy6d467d+8O5i+++OJgvrGxMZi//PLLg/lXX301mL/qqquC+RdffDGYT3ct+V//+tdgft26dcE8J8DjHSgIhw4dGhgYmD59+tgvjYyMBP8vR48eDW5Pd/vFdPmJ+v7pjsonO5/Oye6T7ueTTqb54Ajpv0j3e4yVHx4eziif6e/XbUb/yy8l3Q8n3S8lX150Jzuf7ueT7uec7vtnulPNNJ+uZ6b5TPtPVH6i+qfLp/tz0+1UM92e7s+dqO350idf8id7e7q/b/mez/T1my/bx/nfdfSY/7J/GxgY6OvrG7s903y641F/f/+E5NPJNN/X15fReWlfX9/Yquk+X0zlx278L2fOwXym3z/d522jo6PBfPBt2n/Jp5tvjoyMTMr84OBgXuQ5AR7vQKFra2sLfsZVU1MT/LjpwIEDhw8fHrt9/vz5wbUS+/fvD+7Ici3/iU98YurUqWO3f/DBB8FjxhlnnBFce7J3797gPjrX8osWLQqu5WlpaQmeo5x11lnBtUJ79uwJnoDmWr62tjZ4LvXuu+8GjwKLFy8euzH12WBwe5T8yMjI/v37x24vLi5esGDB+PMlJSXz588fu314ePjAgQO5kz969Ghw9eKUKVNOO+208efLysqCn3Unk8nggoJY+cHBwQ8//HDs9vLy8nnz5o0/X1FRMXfu3LHbBwYGDh48mDv5vr6+4AKNadOmVVdXjz8/ffr04NqHw4cPd3Z2nrz84OBg8OBYUVERPBilews9derU4BuzdPlpx4zd3n9M7uT7+vqCB9Pp06cHD77p8jNmzAge7A4fPhw82MXK9/b2HjlyZOz2ysrK4ELXdPmZM2cG39j39PQERxix8t3d3cGTh6qqquDJRrr8rFmzggf3rq6u4ME9Vr6zszM4lTvllFOCC7E7Ojr+fZpWWlqa+jFOnTo1eHIyMDAQPDnJl3xw55DaPwS353V+dHQ0uLMqKioK7tzyPT8yMhLcGRYXFwd3hvmSLykpCe6ch4eHgzvndHlOgJEcAAAAAGSVe8kBAAAAQFYZyQEAAABAVhnJAQAAAEBWGckBAAAAQFYZyQEAAABAVhnJAQAAAEBWGckBAAAAQFYZyQEAAABAVhnJAQAAAEBWGckBAAAAQFYZyQEAAABAVhnJAQAAAEBWGckBAAAAQFYZyQEAAABAVhnJAQAAAEBWGckBAAAAQFYZyQEAAABAVhnJAQAAAEBWGckBAAAAQFYZyQEAAABAVhnJAQAAAEBWGckBAAAAQFYZyQEAAABAVhnJAQAAAEBWGckBAAAAQFYZyQEAAABAVhnJAQAAAEBWGckBAAAAQFYZyQEAAABAVhnJAQAAAEBW/V8AAAD//4OYZnSQ2DsZAAAAAElFTkSuQmCC)

Each map site (Door, Wall, Room) is an Abstract product. Concrete
factories implement themethods to create instances of concretemap site
types for the samemaze type. An enchanted factory creates amazewith
enchanted rooms, enchanted doors and enchantedwall.

A.1. CREATIONAL PATTERNS 75
Example The following example shows howamazewith bombedmap
sites can be created using a factory.
The following listing shows four interfaces. MapSite defines the Enter
method. The other interfaces embed MapSite – gaining Enter – and list
further methods. That means concrete doors, rooms and walls have to
implement their respectivemethods and Enter aswell.
type MapSite interface {
Enter()
}
type Wall interface {
MapSite
}
type Door interface {
MapSite
IsOpen() bool
SetOpen(isOpen bool)
}
type Room interface {
MapSite
GetSide(direction Direction) MapSite
SetSide(direction Direction, side MapSite)
SetRoomId(roomId int)
GetRoomId() int
}
BombedDoor is a concrete product, an implementation of the Door in-
terface. BombedDoor keeps references to its connecting rooms and knows
if it is open or not.
type BombedDoor struct {
room1 Room
room2 Room
isOpen bool
}

NewBombedDoor instantiates a BombedDoor object and sets its rooms.
isOpen is initialized to false per default.
func NewBombedDoor(room1 Room, room2 Room) *BombedDoor {
return &BombedDoor{room1: room1, room2: room2}
}

76 APPENDIX A. DESIGN PATTERN CATALOGUE
IsOpen and SetOpen are the getter and setter for isOpen.
func (door *BombedDoor) IsOpen() bool {
return door.isOpen
}
func (door *BombedDoor) SetOpen(isOpen bool) {
door.isOpen = isOpen
}
Enter prints amessage according to isOpen.
func (door *BombedDoor) Enter() {
if door.isOpen {
fmt.Println("Enter bombed door")
} else {
fmt.Println("Can’t enter bombed door. Closed.")
}
}
String returns a string describing the door. Themethod name String
is a convention. GO’s formatting package fmt uses the interface Stringer
with String as its onlymethod. The idea being that types implementing
String can be used in the fmt package for formatted printing. String is
the equivalent to Java’s toString.
func (door *BombedDoor) String() string {
return fmt.Sprintf("A bombed door between %v and %v",
door.room1.GetRoomId(), door.room2.GetRoomId())
}

The next listing shows the type Mazewhichmaintains a vector. Vector
is not generic and its elements can be of any type,more precisely any type
implementing the (empty) interface interface. The other twomethods
handle adding and retrieval of rooms fromaMaze object.
type Maze struct {
rooms *vector.Vector
}
func NewMaze() *Maze {
return &Maze{rooms: new(vector.Vector)}
}

A.1. CREATIONAL PATTERNS 77
AddRoom and GetRoom handle adding and retrieval of rooms froma
Maze object.
func (maze *Maze) AddRoom(room Room) {
maze.rooms.Push(room)
}
func (maze *Maze) GetRoom(roomId int) Room {
for i := 0; i < maze.rooms.Len(); i++ {
currentRoom := maze.rooms.At(i).(Room)
if currentRoom.GetRoomId() == roomId {
return currentRoom
}
}
return nil
}
MazeFactory is an interface listing amethod for each Abstract Prod-
uct.
type MazeFactory interface {
MakeMaze() *Maze
MakeWall() Wall
MakeRoom(roomNo int) Room
MakeDoor(room1 Room, room2 Room) Door
}
BombedMazeFactory is an implementation of the MazeFactory in-
terface. MakeMaze returns an instance of a plainmaze. Each of the other
methods return an instance of the bombed product family. MakeWall,
for example, returns an instance of BombedWall. The static types of the

returned objects are of the interface type, but dynamic types are of the
bombed variety. It is transparent to clients of the Make methods which
dynamic type is returned.
type BombedMazeFactory struct{}
func (factory *BombedMazeFactory) MakeMaze() *Maze {
return NewMaze()
}
func (factory *BombedMazeFactory) MakeWall() Wall {
return new(BombedWall)
}

78 APPENDIX A. DESIGN PATTERN CATALOGUE
func (factory *BombedMazeFactory) MakeRoom(roomNo int) Room {
return NewBombedRoom(roomNo)
}
func (factory *BombedMazeFactory) MakeDoor(room1 Room, room2 Room) Door {
return NewBombedDoor(room1, room2)
}
The function CreateMaze creates an instance of Maze and returns a
pointer to the object. The maze has two rooms with a door in between.
The concrete map site types depend the dynamic type of the parameter
MazeFactory (i.e. consider factory being of type BombedFactory, then
the rooms, doors andwallswill be of the bombed product family).
func CreateMaze(factory MazeFactory) *Maze {
aMaze := factory.MakeMaze()
room1 := factory.MakeRoom(1)
room2 := factory.MakeRoom(2)
aDoor := factory.MakeDoor(room1, room2)
aMaze.AddRoom(room1)
aMaze.AddRoom(room2)
room1.SetSide(North, factory.MakeWall())
room1.SetSide(East, aDoor)

room1.SetSide(South, factory.MakeWall())
room1.SetSide(West, factory.MakeWall())
room2.SetSide(North, factory.MakeWall())
room2.SetSide(East, factory.MakeWall())
room2.SetSide(South, factory.MakeWall())
room2.SetSide(West, aDoor)
return aMaze
}
Herewe showhowClients use a bombedmaze factory to create amaze.
var factory *BombedMazeFactory
var maze *Maze
maze = CreateMaze(factory)
maze.GetRoom(1).Enter() //Prints: Can’t enter bombed door. Closed.
Discussion The interfaces Wall, Door and Room embed the MapSite
interface. MapSite’s Enter is added to the embedding interfaces. Embed-

A.1. CREATIONAL PATTERNS 79
ding an interface in another interface is similar to Java’s interface extension.
A default factory type could provide common behaviour for concrete
factories. Concrete factories would embed default factory. The factory
interfacewith the embedded default type emulates abstract types.
The Abstract Factory pattern describes a Create method accepting a
factory object. The method uses the factory to create and return the end
product. From the structure diagram of the Abstract Factory pattern as
described by the GoF it is not apparentwhere thismethod is supposed to
live. Functions in GO are not tied to a type like in Java or C++. Functions
are independent. The Abstract Factory pattern can take advantage of that.
There is no need to introduce a separate type just to hold the Createmethod.
Create can just be function since the receiver of this function would not be

used anyway.

80 APPENDIX A. DESIGN PATTERN CATALOGUE

### A.1.2 Builder

Intent The intention is to abstract steps of construction of objects so that
different implementations of these steps can construct different representa-
tions of objects.
Context Consider the creation of mazes for a fantasy game. Mazes are
complex object structures. Mazes consist ofmap sites like doors, walls and
rooms. Themap sites come in a variety of types like standard, enchanted or
bombed. The creation process ofmazes should be independent of the type
of themap sites and not have to change when new types of doors, walls
and rooms are added.
A solution is a builder interface defining methods for for creation of
parts of the end product. Concrete builders implement the builder interface;
construct and assemble the parts;maintain a reference to the end product
they create; and providemeans for retrieving the end product. A director
construct an object by using the builder interface.

Example In the following we demonstrate how a maze with standard
map sites can be created using a builder.
The MazeBuilder interface is the common builder interface concrete
builders implement. The method GetMaze provides access to the end
product the builder creates and there are buildmethod for each part.
type MazeBuilder interface {
GetMaze() *Maze
BuildMaze()
BuildRoom(roomId int)
BuildDoor(roomId, room2 int)
}
This listing shows the declaration of StandardMazeBuilder which
has a reference to a Maze object (the end product). GetMaze provides
public access to the end product.

A.1. CREATIONAL PATTERNS 81
type StandardMazeBuilder struct {
maze *Maze
}
func (builder *StandardMazeBuilder) GetMaze() *Maze {
return builder.maze
}
BuildMaze, BuildRoom and BuildDoor each create amap site in the
standard variety. BuildMaze initializes the end product with a newmaze
object. BuildRoom instantiates a new roomif the roomdoes not exist. The
room gets four standard walls. BuildDoor retrieves the rooms that the
1
door is to connect. A new standard door is is inserted at each room. Note
that each build function operates on the same maze object. Each build step
contributes to the creation of awhole end product.
func (builder *StandardMazeBuilder) BuildMaze() {
builder.maze = NewMaze()
}
func (builder *StandardMazeBuilder) BuildRoom(roomId int) {
if builder.maze.GetRoom(roomId) == nil {
room := NewStandardRoom(roomId)
builder.maze.AddRoom(room)

room.SetSide(North, new(StandardWall))
room.SetSide(East, new(StandardWall))
room.SetSide(South, new(StandardWall))
room.SetSide(West, new(StandardWall))
}
}
func (builder *StandardMazeBuilder) BuildDoor(roomId1 int, roomId2 int) {
room1 := builder.maze.GetRoom(roomId1)
room2 := builder.maze.GetRoom(roomId2)
door := NewStandardDoor(room1, room2)
room1.SetSide(West, door)
room2.SetSide(East, door)
}
BuildMaze uses a MazeBuilder object to create a maze. The map
site types of the maze depend on the dynamic type of the builder object.
Themethod takes an object of type MazeBuilder and calls the builder’s
1
For simplicity the door replaces the east and the west wall without checking the
validity of that operation.

82 APPENDIX A. DESIGN PATTERN CATALOGUE
Buildmethods. Each step of BuildMaze adds a part to the end product
which the builder object keeps track off.
func BuildMaze (builder MazeBuilder) *Maze {
builder.BuildMaze()
builder.BuildRoom(1)
builder.BuildRoom(2)
builder.BuildDoor(1, 2)
return builder.GetMaze()
}
The code belowshows howclients use a builder to create amazewith
standardmap sites. A builder gets instantiated; the builder object is passed
to BuildMaze,which creates themaze calling builder’s Buildmethods;
maze is assigned themaze builder created; and finally maze’s structure
is printed to the console.
builder:= NewStandardMazeBuilder()
BuildMaze(builder)
maze := builder.GetMaze()
fmt.Print(maze)
Discussion Design Patterns describes a distinct Director type with a Con-
structmethod. This implementation of Builder omits the separate Director

type. The task of a Director is to “construct an object using the Builder
interface”[39]. In GO, a distinct Director type is not necessary. Direc-
tors don’t store state of the product nor the builder they use to create the
product. Director is amere vessel for Constructmethods. The Director’s
Construct method could be a function making the Director type super-
fluous. However, a Director type could be used to group different Build
methods together.
If common functionality between builders is identified, a default builder
type should be implemented. The default builder could alsomaintain the
reference to the product it creates. Concretemaze builderswould embed
the default builder to gain the common functionality and the reference to
the product.

A.1. CREATIONAL PATTERNS 83

### A.1.3 FactoryMethod

Intent Define an interface for creating an object, but let subclasses decide
which type to instantiate.
Context Consider a frameworks for documents. Applicationswill have
to use different types of documents like resumes or reports and each type
of document will have different pages. Adding new types of documents
will complicate the creation of documents, since the framework doesn’t
knowwhich concrete document type it should instantiate.
The Factorymethod offers a solution. In the document interface ame-
thod for creating the pages is defined and concrete documents implement
this method. Letting subtypes decide how to create the pages of a doc-
ument encapsulates that knowledge out moves it out of the framework.
The page creatingmethod is the factorymethod because it “manufactures”
objects.
The FactoryMethod pattern can be implemented by usingmethods or

by using functions. We discuss both alternatives.
Example - Factory methods In this example we describe the implementa-
tion of document-creating factorymethods.
Document defines the common interface. CreatePages is the factory
method concrete document types implement.
type Document interface {
CreatePages()
}
DefaultDocument provides common behaviour for document sub-
types. Concrete documents can embed DefaultDocument to inherit the
member Pages, a reference to the pages of a document. Pages is a public
vector, to grant DefaultDocument-embedding types access, evenwhen
they are defined outside of DefaultDocument’s package. The String
method returns the document’s pages concatenatedwith a comma.

84 APPENDIX A. DESIGN PATTERN CATALOGUE
type DefaultDocument struct {
Pages *vector.StringVector
}
func NewDefaultDocument() *DefaultDocument {
return &DefaultDocument{new(vector.StringVector)}
}
The Stringmethod returns the document’s pages concatenatedwith a
comma.
func (this *DefaultDocument) String() string {
var result string
for i := 0; i < this.Pages.Len(); i++ {
page := this.Pages.At(i)
result += fmt.Sprintf("%v, ", page)
}
return result
}
Resume and Report are implementations of the interface Document.
Both types define CreatePages methods, adding pages (here simple
strings) to their Pages member they gained from embedding Default-
Document.
type Resume struct {
*DefaultDocument
}
func NewResume() *Resume {
return &Resume{NewDefaultDocument()}
}
func (this *Resume) CreatePages() {
this.Pages.Push("personal data")
this.Pages.Push("education")
this.Pages.Push("skills")
}

type Report struct {
*DefaultDocument
}
func NewReport() *Report {
return &Report{NewDefaultDocument()}
}

A.1. CREATIONAL PATTERNS 85
func (this *Report) CreatePages() {
this.Pages.Push("introduction")
this.Pages.Push("background")
this.Pages.Push("conclusion")
}
Depending on the dynamic type of the Document object doc theme-
thod CreatePages behaves differently.
var doc Document
doc = NewResume()
doc.CreatePages()
fmt.Println(doc)
doc = NewReport()
doc.CreatePages()
fmt.Println(doc)
Example - Factory functions A problem with factory methods is that a
new type is necessary to create the appropriate product objects. We take
advantage of GO’s function type func.
In the following examplewe look at the same problemas above: Cre-
ation of documentswith different pages. But instead of defining separate
types for object creation we declare functions encapsulating the creation
process.
Document maintains a reference to its pages. The method Create-

Pages create the pages. CreatePages’ parameter is a function returning
a pointer to a Vector. CreatePages calls the passed function create-
Pages and assigns the return value to pages. Themethod CreatePages
accepts functions that have no parameters and return a pointer to a vector
object (func() vector.Vector). Two examples of functions that can
*
be passed to CreatePages are shown in the next listing.
type Document struct {
Pages *vector.Vector
}
func (this *Document) CreatePages(createPages func() *vector.Vector) {
this.Pages = createPages()
}

86 APPENDIX A. DESIGN PATTERN CATALOGUE
CreateResume and CreateReport create a vector, add the necessary
pages, and return the result. Bothmethods have a signature that is accepted
by Document’s CreatePages.
func CreateResume() *vector.Vector {
pages := new(vector.Vector)
pages.Push("personal data")
pages.Push("education")
pages.Push("skills")
return pages
}
func CreateReport() *vector.Vector {
pages := new(vector.Vector)
pages.Push("introduction")
pages.Push("background")
pages.Push("conclusion")
return pages
}
The following listing shows how the factory functions CreateResume
and CreateReport are used to create the pages of a Document object
doc.
var doc Document = new(Document)
doc.CreatePages(CreateResume)
fmt.Println(doc)
doc.CreatePages(CreateReport)
fmt.Println(doc)
}
Discussion The first example is an implementation of the classic Factory

Method. An interface for product creation that subtypes implement. The
second example uses functions to avoids having to define a new type in
order to create different products.
In the first example a type is used to provide default behaviour. This
type ismeant to be embedded, not to be instantiated. GO does not provide
amechanismto avoid instantiating types.

A.1. CREATIONAL PATTERNS 87

### A.1.4 Prototype

Intent Create objects by cloning a prototypical instance.
Context Consider again mazes for a fantasy game and the creation of
mazes withmixedmap site types (bombed, enchanted or standard). Ab-
stract Factory creates objects of the same family.
The Prototype pattern offers a solution. The products have a common
interface for cloning itself. Concrete products implement that interface
to return a copy of itself. New instances of the products are created by
asking the prototypical products to clone themselves. This approach can
be combined with the Abstract Factory pattern. The factory maintains
references to prototypical objects and depending on the dynamic type of
the prototype objects the factory clones the prototype object, and initialises
and returns it. The dynamic type of the object determines what type of
product the factory is to create.

Example Once again we employ the example of creating mazes for a
fantasy game. This timewe use a prototype factory to createmazeswith
mixed typemap sites (bombed rooms, enchanted doors and standardwalls).
All prototypical objects have to implement a Clone method, returning a
copy of themselves.
MazePrototypeFactorymaintains references to amaze and to pro-
totypes of map sites. NewMazePrototypeFactory constructs a maze
prototype factory objectwith the passed prototypical objects.
type MazePrototypeFactory struct {
prototypeMaze *Maze
prototypeRoom Room
prototypeWall Wall
prototypeDoor Door
}
func NewMazePrototypeFactory(
maze *Maze, room Room, wall Wall, door Door) *MazePrototypeFactory {
this := new(MazePrototypeFactory)

88 APPENDIX A. DESIGN PATTERN CATALOGUE
this.prototypeMaze = maze
this.prototypeRoom = room
this.prototypeWall = wall
this.prototypeDoor = door
return this
}
To usemap sites with the prototype factory we add the Clonemethod
to the MapSite interface,which is described in Appendix A.1.1. We can’t
put the Clonemethod inMapSite, sincewewant to avoid having to cast the
returned object. Each Product type interface has to declare its own Clone
methodwith the return value type being of the right product type.
type MapSite interface {
//snip
Clone() Door
}
The Makemethods are factorymethods used in the template “method”
CreateMaze (see Appendix A.1.1). The factorymethods copy the facto-
ries prototypes by calling Clone on them. MakeDoor and MakeRoom do
further initialization of the cloned prototype. Clone for EnchantedDoor
is shown in the next listing.

func (this *MazePrototypeFactory) MakeMaze() *Maze {
return this.prototypeMaze.Clone()
}
func (this *MazePrototypeFactory) MakeWall() Wall {
return this.prototypeWall.Clone()
}
func (this *MazePrototypeFactory) MakeDoor(room1, room2 Room) Door {
door := this.prototypeDoor.Clone()
door.SetRooms(room1, room2)
return door
}
func (this *MazePrototypeFactory) MakeRoom(roomId int) Room {
room := this.prototypeRoom.Clone()
room.SetRoomId(roomId)
return room
}

A.1. CREATIONAL PATTERNS 89
Clone instantiates a newEnchantedDoor object and reassigns its rooms
and if it is open or not.
func (this *EnchantedDoor) Clone() Door {
door := new(EnchantedDoor)
door.room1 = this.room1
door.room2 = this.room2
door.isOpen = this.isOpen
return door
}
Discussion Implementing the Clonemethod correctly is – like inmany
other languages – the the hardest part, especiallywhen the object contain
circular references. Copymethods could beworking like C++’s copy con-
structors, but that still doesn’t solve the problemof shallow versus deep
copying. A solution would be the serialization of objects into streams of
bytes. GO’s gob package manages binary values exchanged between an
Encoder (transmitter) and a Decoder (receiver). Objects would be Encoded
into a stream of bytes, sent to aWriter object and received by an Reader
and the decoded by a Decoder.

90 APPENDIX A. DESIGN PATTERN CATALOGUE

### A.1.5 Singleton

Intent Ensure that only a single instance of a type exists in a program,
and provides a global access point to that object.
Context Consider a registry for services. There should always be only
one instance of a registry per application.
The Singleton pattern provides a solution by providing a single way to
create and get hold of the object. The type which should be instantiated
needs to be a package private type. A public function ensures that there is
only one instance of the private type at a time. The private type can define
publicmembers to give clients access to its state and functionality.
Example - Package encapsulates Singleton The following example out-
lines the implementation of a registry ofwhichwe onlywant to have one
instance. The following two listings reside inside the package registry.
The struct registry cannot be named outside the registry package,

so it can only be instantiated inside the package. The variable instance
is a reference to the only instantiation of the type registry.
package registry
type registry struct {
state *State
}
var instance *registry
The public function Get provides access to the singleton instance,
creating a newobject if none existswhen it is called.
func Get() *registry {
if instance == nil {
instance = &registry{}
}
return instance
}
func (this *registry) GetState() *State { ... }

A.1. CREATIONAL PATTERNS 91
The function Get checks if instance is nil (nil is GO’s null pointer).
Get returns the pointer of the object stored in instance. Only if in-
stance is nil, a newinstance is created, otherwise the existing onewill
be returned. Get uses lazy initialization. The object instance is only
initialized when needed. The alternative would be to initialize instance
on package import:
var instance = new(registry)
The only way for clients to get hold of an instance of type regis-
try.registry is by calling registry.Get. Consider the following
code examples outside of the package registry:
aRegistry := registry.Get()
The following does notwork:
aRegistry := new(registry.registry)
Even though the type registry is not visible, its method GetState is.
Clients can use the public functions and publicmembers of private types:
aRegistry.GetState()

For documentation purposes the registry package could define a
public interface Registry listing registry’s publicmethods.
Example - Package as Singleton As an alternative to encapsulating a
singleton type in a package, packages themselves could be used as single-
tons. Singleton-types become packages; singleton-typemembers become
static variables;methods become functions. Consider the following code
example:
package registry
var state *State
func init() {
state = new(State)
}

92 APPENDIX A. DESIGN PATTERN CATALOGUE
The package registry itself acts as singleton. The singleton’s state is kept
in static variables, here in state of type State. The object initialization is
nowdone in the init function instead of in Get.
func GetState() *State {
return state
}
Packages cannot be instantiated, therefore there is no need to control or
limit instantiation. Clients can use the singleton’s functionality by import-
ing the package and interactingwith it directly:
import (
registry1 "registry"
registry2 "registry"
)
Packages can be importedmultiple times. The package registry is im-
ported twicewith two different names, but both names refer to the same
“instance” of the package. The initmethod is executed only once, even
though there are two references to the same package.
registry1.GetState() == registry2.GetState() // true

The aliases registry1 and registry2 refer to the same package. Calling
GetState on either returns the same object.
Discussion The Singleton pattern is usually implemented by hiding (us-
ing scope private) the singleton’s constructor. In GO, constructors cannot
be made private. The examples above show two ways to limit object in-
stantiation by using GO’s package accessmechanisms. The first option is to
declare the singleton type’s scope as package. Clients outside the package
can’t create instances of the un-exported type. The other option is to use a
package as singleton. Packages can’t be instantiated and package initial-
ization is only done once. The static package variables are the singleton’s
state.
The Singleton pattern is only effective for clients outside the package
a singleton is declared. Within the singleton’s packagemultiple instances

A.1. CREATIONAL PATTERNS 93
can be created, since GO does not have a private scope; in particular, this
might be done inadvertently by embedding registry.

94 APPENDIX A. DESIGN PATTERN CATALOGUE

### A.2 Structural Patterns

Structural design patterns identify simple ways to realize relationships
between entities.

### A.2.1 Adapter

Intent Translate the interface of a class into a compatible interface. An
adapter allows classes to work together that normally could not because of
incompatible interfaces.
Context Consider a framework for animals. The core of the framework is
an interface defining the behaviour of animals. Now,wewant to incorpo-
rate a third-party library that provides reptiles, but the problemis that the
reptiles can’t be used directly because of incompatible interfaces.
The solution is to provide a type (the Adapter) that translates calls to its
interface (the Target) into calls to the original interface (the Adaptee). The
diagrambelowshows this structure.

![](data:image/png;base64,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)

We showtwo alternatives of implementing the adapter pattern in GO.
The first example uses embedding and the second object composition.
Example - Embedding In the following examplewe adapt aReptile object
to the Animal interface.
Animal is the Target interface that concrete animals have to implement.

A.2. STRUCTURAL PATTERNS 95
package animals
type Animal interface {
Move()
}
The type Cat is a concrete animal, since it implements the method
Move.
type Cat struct { ... }
func (this *Cat) Move() { ... }
The package reptiles provides the type Crocodile. Crocodile has
themethod Slither. Crocodile is the Adaptee. Wewant be able to use
Crocodilewhere an Animal is required. We need to adapt the Slither
method to a Movemethod.
package reptiles
type Crocodile struct { ... }
func (this *Crocodile) Slither() { ... }
The struct CrocodileAdapter adapts an embedded crocodile so that
it can be used as an Animal. CrocodileAdapter objects implicitly im-
plement the Animal interface. Clients of CrocodileAdapter can call
Move as well as Slither and any other public behaviour reptiles.
Crocodilemay provide.
package animal
type CrocodileAdapter struct {
*reptiles.Crocodile
}

func NewCrocodile() *CrocodileAdapter {
return &CrocodileAdapter{new (reptiles.Crocodile)}
}
func (this *CrocodileAdapter) Move() {
this.Slither()
}

96 APPENDIX A. DESIGN PATTERN CATALOGUE
Clients of package animals can use Cats and Crocodileswhere an
object of type Animal is required. It is transparent to clients that Croco-
dile’s implementation is provided by a third-party library.
import "animals"
...
var animal animals.Animal
animal = new(animals.Cat)
animal.Move()
animal = animals.NewCrocodile()
animal.Move()
Example - Composition Alternatively,we could use composition rather
than embedding in our adapter object. The following listing highlights the
necessary changes.
CrocodileAdapter has a reference to a reptiles.Crocodile ob-
ject. Aswith embedding the object still has to be initialized, thereforewe
still need NewReptile().
type CrocodileAdapter struct {
cocodile *reptiles.Crocodile
}
func (this *CrocodileAdapter) Move() {
this.crocodile.Slither()
}
Discussion Both examples we showed here are object adapters, even the

embedding example, since the embedded type is actually an object.
Having the choice between embedding and composition is a similar to
class-based languages where an adapter can use inheritance or composi-
tion. Composition should be used when the public interface of the adaptee
should remain hidden. Aminor drawback of composition is that the call of
Slither needs to be fully quantified (this.crocodile.Slither()).
Embedding exhibits the adaptee’s entire public interface but reduces book-
keeping (this.Slither()).

A.2. STRUCTURAL PATTERNS 97
The embedded type reptiles.Crocodile of CrocodileAdapter
has to be initialized (to avoid nil reference errors), thereforewe need New-
Crocodile. The initialization of embedded types is an additional burden
wewould like to avoid.

98 APPENDIX A. DESIGN PATTERN CATALOGUE

### A.2.2 Bridge

Intent Avoid permanent binding between an abstraction and an imple-
mentation to allowindependent variation of either.
Context Consider a framework for drawing shapes. There are different
shapes and have to be displayed inmultiple ways: bordered, unbordered
etc. A permanent binding between the shape and howthey are displayed
should be avoided so that the two can vary independently.
The bridge pattern offers a solution, by separating abstraction from
implementation. The shapes are the abstractions and howthey are rendered
is the implementation.
Example In this examplewe decouple shapes (the abstractions) fromtheir
drawing API (the implementation) by implementing the Bridge pattern.

![](data:image/png;base64,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dXX1rVu3hg4dOmzYMJIkv/jii61bt0J9qmtfX98JEyZAZDUQrK6uzsfHB3p8/fq1vr4+pGWVSCTdu3eHUM3a2towFnd39xMnTkBNBweHuLg4KARrlI67u/v58+epXfi0r7KyUm4sigMhSVLVWM6dO9etW7e6ujrFiaAGKHehxo4du3LlSugCMqxMmTLl5MmTcAhOqa2thdjKkPBp6dKlVLOOjo6QrVYV7RApDUEQoAEV/O677/7555+w3bVr16dPn8K2p6fnvXv3SJK0srLKz88Htevh4eHv73/8+HGJREJv8Nq1a15eXtSug4NDYmIinPvgwQOq/MCBA++9956rq6uLiwvox27duhUVFU2cOPH27duurq5SqbRHjx5U41TXFhYWcoLJ9Whvb5+enk6S5JIlSz7//HMo7Nat2927dysrK3V0dPr27evu7v7ee+/FxMSQJAmFOTk59FFA4evXr+kXqkePHpQw1FgUBwLdKR3L6tWrp06dqnRuqAHSG7927Rp8z02na9eur169SkhI4PF4/f+Dvb09/E2am5tTabXAHpcbmhyoghGk3VD3gXJtbe3Dhw/d3Nwgn11NTY2joyNECn7y5ImLi8vr16+1tbUhnhmLxbp79+6NGzdOnDixaNGiZ8+eGRgYQDtJSUlUNMs3b97k5OT079//9evXBEE4OztD+c8//7xv377Tp09bWVklJCQsWLDAwMDAzMzs1q1bMpls0KBB5eXlP/7444wZM8CJTHWdnZ3NZrPlBNu5cyfVY2lpqUQicXBwSE9P/+6778zMzCCuW2Fh4f3797W0tIyNjeWC5D948IDP58u9jnvw4IGJiYmtrS1Vsnv3bghYQR+L0oEQBKFqLNra2ooZoegDlLtQSUlJch8uFhQUVFdXd+3aNS4u7l//+te3335LP5qXl1dZWdm3b1/YvXnzZrdu3RTfNCIIwgjqfMGvX7+GPP50byMoo549e+rp6SUlJVGpMTIyMths9ogRI7744ouKigp6O6ampsnJyTKZrKamJjIy0snJSVdXNykpiWoQ2nRxcbGysiouLv73v/8Nh0xNTb/66ivILy2Tyfbs2UMll6O6prdDCWZqavrXX3/V1tbKZLLPPvvM1dWVxWItWLBg69at2dnZL+pZuHBhSkqKnBhU4+odwUKh8LPPPsvLy4P0LXIyKA5EzVgmTpx47do1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)

The above diagram shows the structure of the Bridge pattern we are
using in this example. Abstractions (here Shapes) use the Implementor in-
terface (here DrawingAPI). Shapes are not concerned howthe DrawingAPI
is implemented. A CircleShape maintains a reference to a DrawingAPI
object by embedding DefaultShape. The dynamic type of the DrawingAPI

A.2. STRUCTURAL PATTERNS 99
object determines if the shape will be drawn as a filled or empty figure.
This decouples the implementation details fromthe abstraction.
The following listings showhowthis can be implemented in GO.
type Shape interface {
Draw()
ResizeByPercentage(pct float64)
}
type DefaultShape struct {
drawingAPI DrawingAPI
}
func NewDefaultShape(drawingAPI DrawingAPI) *DefaultShape {
return &DefaultShape{drawingAPI}
}
The Shape interface defines two methods that all concrete shapes have
to implement. The type DefaultShape ismeant to be embedded by all
concrete shapes andmaintains a reference to a DrawingAPI object of inter-
face type. It is transparent to concrete shapes how the API is implemented.
DefaultShapewould also hold common behaviour for shapes.
type CircleShape struct {
x,y,radius float64
*DefaultShape
}

func NewCircleShape(x,y,radius float64, drawingAPI DrawingAPI) *CircleShape{
this := new(CircleShape)
this.x, this.y ,this.radius = x, y, radius
this.DefaultShape = NewDefaultShape(drawingAPI)
return this
}
CircleShape is a concrete shape with coordinates of its center; a radius;
and it embeds DefaultShape, inheriting a reference to a DrawingAPI object.
NewCircleShape is a constructormethod initializing the location, radius
and the embedded type.
func (this *CircleShape) Draw() {
this.drawingAPI.DrawCircle(this.x, this.y, this.radius)
}

100 APPENDIX A. DESIGN PATTERN CATALOGUE
func (this *CircleShape) ResizeByPercentage(pct float64) {
this.radius *= pct
}
Draw is the implementation specificmethod,making use of the implemen-
tation object. Draw calls the DrawCircle on its drawingAPI object. The
dynamic type of drawingAPI determines how the circle will be drawn.
ResizeByPercentage changes the circle’s radius and abstraction spe-
cific, only operating on the abstraction, not on the drawing API.
type DrawingAPI interface {
DrawCircle(x,y,radius float64)
}
type FilledFigure struct {}
func (this *FilledFigure) DrawCircle(x,y,radius float64) {
//draw a filled circle
}
type EmptyFigure struct {}
func (this *EmptyFigurs) DrawCircle(x,y,radius float64) {
//draw an empty circle
}
DrawingAPI is themain interface for implementations. Concrete imple-
mentations of DrawingAPI implement DrawCircle. The example above

implements two concrete drawing APIs with different implementations of
DrawCircle.
func main() {
filledShape := NewCircleShape(1, 2, 3, new(FilledFigure))
emptyShape = NewCircleShape(5, 7, 11, new(EmptyFigure))
filledShape.ResizeByPercentage(2.5)
filledShape.Draw()
emptyShape.ResizeByPercentage(2.5)
emptyShape.Draw()
}
The above listing shows how clients can use this implementation of the
Bridge pattern. Two shape objects get instantiated, the abstractions. The

A.2. STRUCTURAL PATTERNS 101
interesting part is that they get initializedwith different Implementation
objects, thus changing howtheir Drawmethod draws the shapes.
Discussion Shape in the original pattern is an abstract class to define the
interface and tomaintain a reference to a DrawingAPI object. GO doesn’t
have abstract classes, sowe use the combination of an interface (Shape) and
provide a default type (DefaultShape) with that reference (and common
functionality if necessary). Alternatively each refined abstraction could
maintain their own reference. The advantage of embedding DefaultShape
become apparent when DefaultShape implemented common functionality
for refined abstractions.

102 APPENDIX A. DESIGN PATTERN CATALOGUE

### A.2.3 Composite

Intent Treat a group of objects in the sameway as a single instance of an
object and create tree structures of objects.
Context Consider the assembly of car. A car is made of parts and each
part is mad of other parts in turn. A car has an engine, an engine has
pistons, and so on. Parts can be composed to formlarger components. The
problemis that we don’t want to treat simple parts and parts that aremade
of other parts differently.
TheComposite pattern offers a solution. The key is to define an interface
for all components (primitives and their containers) to be able to treat them
uniformly.

![](data:image/png;base64,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)

All elements of the structure (Leafs as well as Composites) implement
the Component interface. The interface Composite is for components that
are made of sub components, which in turn can be Composites. Default

A.2. STRUCTURAL PATTERNS 103
Composite provides common functionality for Concrete Composites. The
method Operation in Default Composite is called recursively on all of its
components. Concrete Composites embed Default Composite and provide
their implementation of Operation, which is called by Default Composite’s
method Operation.
Example Herewe look at the car examplementioned above. Cars have
engines and tires; engines have pistons; tires have tubes. Each of the car’s
components is a part. Parts that are assemblies of other parts are composite
parts. At the end of the assembly process we ask the car to check all of
its parts if they areworking correctly, and the sub partswill ask their sub
parts.
Part is the main interface of all components of a car. All elements
(nodes and leaves) of the composite structurewill have to implement the
CheckFunctionalitymethod.
type Part interface {

CheckFunctionality()
}
Piston implements the Part interface and is a Leaf: Piston doesn’t have
sub parts. There are other parts like Tubeswe omitted for brevity.
type Piston struct {}
func NewPiston() *Piston {
return new(Piston)
}
func (this *Piston) CheckFunctionality() {
fmt.Println("up and down")
}
To bundle all functionality that is common to parts that have sub parts,
we define the CompositePart interface. CompositePart lists the Add
and Removemethods and embed the Part interface, adding the Check-
Functionality method to the interface. Implementations of Compos-
itePartwill have to implement all fourmethods.

104 APPENDIX A. DESIGN PATTERN CATALOGUE
type CompositePart interface {
Add()
Remove()
Part
}
DefaultCompositePart defines common functionality for compos-
ite parts and ismeant to be embedded. DefaultCompositePartmain-
tains a vector to store the sub parts.
type DefaultCompositePart struct {
parts *vector.Vector
}
func NewDefaultCompositePart() *DefaultCompositePart {
return &DefaultCompositePart{parts:new(vector.Vector)}
}
The vector parts can bemanipulated with the Add and Removemeth-
ods. This functionality is common to all composite parts.
func (this *DefaultCompositePart) Add(part Part) {
this.parts.Push(part)
}
func (this *DefaultCompositePart) Remove(part Part) {
for i := 0; i < this.parts.Len(); i++ {
currentPart := this.parts.At(i).(Part)
if currentPart == part {
this.parts.Delete(i)
}
}
}
CheckFunctionality recursively calls CheckFunctionality on

all of its parts (which can in turn be of type CompositePart).
func (this *DefaultCompositePart) CheckFunctionality() {
for i := 0; i < this.parts.Len(); i++ {
currentPart := this.parts.At(i).(Part)
currentPart.CheckFunctionality()
}
}
In our example cars are composite parts enabling cars to contain parts.
Car implements CompositePart since it embeds DefaultComposite-

A.2. STRUCTURAL PATTERNS 105
Part. CheckFunctionality overrides the embedded DefaultCom-
positePart.CheckFunctionality. CheckFunctionality prints a
message on the console and then calls CheckFunctionality of the em-
bedded type. The implementation of Engine and Tire are similar to Car,
but omitted for brevity.
type Car struct {
*DefaultCompositePart
}
func NewCar() *Car {
return &Car{NewDefaultCompositePart()}
}
func (this *Car) CheckFunctionality() {
fmt.Println("move")
this.DefaultCompositePart.CheckFunctionality()
}
The following creates a composite object structure: an enginewith two
pistons, two tireswith a tube, and a carwith that engine and the two tires.
CheckFunctionality called on car traverses the structure and each part
prints a statusmessage.
car := NewCar()
engine := NewEngine()
tire1 := NewTire()
tire2 := NewTire()
engine.Add(NewPiston())
engine.Add(NewPiston())

tire1.Add(NewTube())
tire2.Add(NewTube())
car.Add(engine)
car.Add(tire1)
car.Add(tire2)
car.CheckFunctionality()
Discussion Like in Java, interfaces can be extended in GO too. This is
done by one interface embedding another interface. Interfaces can embed
multiple interfaces. In the example above the interface CompositePart

106 APPENDIX A. DESIGN PATTERN CATALOGUE
embeds the interface Part and “inherits” all of Part’smethods.
Composite objects have functionality in common, like Add or Remove.
Since GO has no abstract classes to encapsulate common functionality a
separate type doing just that has to be implemented. Types wishing to
inherit from the “abstract” type have to embed this default type. In the
above example the default type DefaultCompositePart implemented
all of CompositePart’s methods. Types embedding DefaultCompo-
siteType automatically fulfil the CompositePart and therewith the
Part interface.
Car’s CheckFunctionality overrides the CheckFunctionality
method of the embedded type DefaultCompositeType. However, it is
still possible to call the overloaded function by fully qualifying themethod.
An embedded type is actually just an ordinary object of the embedded

type. The object can be accessed by its type name. This enabled us to call
DefaultCompositeParts’s CheckFunctionality inside the overrid-
ingmethod, akin to a super call in Java:
aConcreteComposite.DefaultCompositePart.CheckFunctiona-
lity()

A.2. STRUCTURAL PATTERNS 107

### A.2.4 Decorator

Intent Augment an object dynamicallywith additional behaviour.
Context Consider a window based user interface. Depending on their
content,windows can either have horizontal, vertical, none or both scroll-
bars. The addition and removal of scrollbars should be dynamic and
windows should not be concerned about scrollbars. The scrollbars should
not be part of thewindow.
The Decorator pattern offers a solution. Windows are going to be
decorated with scrollbars. The image below shows the structure of the
Decorator pattern.

![](data:image/png;base64,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)

A Decorator (window decorator) wraps the Component (window) and
adds additional functionality (scrollbars). The Decorator’s interface con-
forms to the Components’s interface. The Decoratorwill forward requests
to the decorated component. Components can be decorated withmultiple
decorators. This allows independent addition of horizontal and vertical

108 APPENDIX A. DESIGN PATTERN CATALOGUE
scrollbars to windows. It is transparent to clients working with Compo-
nents if the Component is decorated or not.
Example In this examplewe implement the decoration of a simplewin-
dowwith horizontal an vertical scrollbars.
Window is the common interface for Components. Windows and decora-
tors have to implement Draw and GetDescription.
type Window interface {
Draw()
GetDescription() string
}
SimpleWindow is a Concrete Component, an implementation of theWin-
dow interface. SimpleWindow is oblivious that it is going to be decorated
with scrollbars.
type SimpleWindow struct {}
func (window *SimpleWindow) Draw() {
//draw a simple window
}
func (window *SimpleWindow) GetDescription() string {
return "a simple window"
}
WindowDecorator is the Decorator and maintains a reference to a

object of the interface type Window. All window decorators have to im-
plement theWindowinterface. To provide default functionality Window-
Decorator implements Window and forward calls to its window object.
WindowDecorator ismeant to be embedded by concrete window decora-
tors.
type WindowDecorator struct {
window Window
}
func NewWindowDecorator(window Window) *WindowDecorator {
return &WindowDecorator{window}
}

A.2. STRUCTURAL PATTERNS 109
func (this *WindowDecorator) Draw() {
this.window.Draw()
}
func (this *WindowDecorator) GetDescription() string {
return this.window.GetDescription()
}
VerticalScrollBars is a Concrete Decorator and embeds Window-
Decorator. Thus, it automatically implements the Window interface.
VerticalScrollBars overrides WindowDecorator’s methods. Draw
draws vertical scrollbars (actually it prints amessage on the console) and
then draws the actual window. GetDescription appends some text to
the decoratedwindow’s description.
type VerticalScrollBars struct {
*WindowDecorator
}
func NewVerticalScrollBars(window Window) *VerticalScrollBars{
this := new(VerticalScrollBars)
this.WindowDecorator = NewWindowDecorator(window)
return this
}
func (this *VerticalScrollBars) Draw() {
this.drawVerticalScrollBar()
this.window.Draw()
}

func (this *VerticalScrollBars) drawVerticalScrollBar() {
fmt.Println("draw a vertical scrollbar")
}
func (this *VerticalScrollBars) GetDescription() string {
return this.window.GetDescription() + ", including vertical scrollbars"
}
There is also a HorizontalScollBars type. The implementation is
omitted for brevity.

110 APPENDIX A. DESIGN PATTERN CATALOGUE
This code creates a window with vertical and horizontal scrollbars and
prints the description.
var window = NewHorizontalScrollBars(
NewVerticalScrollBars(
new(SimpleWindow)))
fmt.Println(window.GetDescription())
Discussion The Decorator pattern in GO is implemented with compo-
sition. The Decorator maintains a reference to the object that is to be
decorated. Decorators should not embed the type to be decorated. Window-
Decorator could embed SimpleWindow, but every time a newwindow
type is added a new decorator type is needed too. This would cause
an “explosion” of types, which the Decorator pattern is trying to avoid.
WindowDecorator had to embed Window, but the problemis Window is
an interface and only types can be embedded in structs, but not interfaces.
WindowDecorator has to implement allmethods defined in Window

and forward calls to its member window. Concrete window decorators
embed WindowDecorator and override the necessary methods. Since
there are no abstract types in GO, WindowDecorator can be instanti-
ated and Window objects could be decorated with a WindowDecorator.
This cannot be avoided by limiting WindowDecorator’s visibility to pack-
age. Decorators for Windows are likely to be provided outside of Window-
Decorator’s package. Decorators embed WindowDecorator and need
therefore to instantiate it.

A.2. STRUCTURAL PATTERNS 111

### A.2.5 Fac¸ade

Intent Define a high-level abstraction for subsystems and provide a sim-
plified interface to a larger body of code tomake themeasier to use.
Context Consider a highly complex systemof objects representing com-
puter components like CPU,memory or hard drives. It is desirable to be
able to use the computer systems having to knowthe sub systems.

![](data:image/png;base64,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)

Clients can be shielded fromthe low-level details by providing a higher-
level interface. To hide the implementation of the Subsystems a Fac¸ade type
(computer) supplies a unified interface. Most Clients interact with the
Fac¸adewithout having to knowabout its internals. The Fac¸ade pattern still
allowaccess to lower-level functionality for the fewClients that need it.
In GO the Fac¸ade pattern can be implemented as its own type or a
package could be used to represent a Fac¸ade. We showboth alternatives.
Example - Fac¸ade as type In the following example we hide the CPU,
memory and hard drive subsystems behind a computer fac¸ade.
In the following listing the three types CPU,Memory andHardDrive
are the complex Subsystemswe are providing a unified interface for. Clients
will not have to access the subsystems, but can if they need to. Themethods

here are simplistic for the sake of understandability.
type CPU struct {}
func (this *CPU) Freeze() {
fmt.Println("CPU frozen")

112 APPENDIX A. DESIGN PATTERN CATALOGUE
}
func (this *CPU) Jump(position int64) {
fmt.Println("CPU jumps to", position)
}
func (this *CPU) Execute() {
fmt.Println("CPU executes")
}
type Memory struct {}
func (this *Memory) Load(position int64, data []byte) {
fmt.Println("Memory loads")
}
type HardDrive struct {}
func (this *HardDrive) Read(lba int64, size int64) []byte {
fmt.Println("HardDrive reads")
return nil
}
The type Computer is the Fac¸ade and maintains a references to the
subsystems.
type Computer struct{
cpu *CPU
memory *Memory
hardDrive *HardDrive
}
func NewComputer() *Computer {
cpu := new(CPU)
memory := new(Memory)
hardDrive := new(HardDrive)
return &Computer{cpu,memory,hardDrive}
}
Themethod Start knows how the subsystems work together. Compu-
ter shields the client fromhaving to knowabout the innerworkings of the
subsystem.
func (this *Computer) Start() {

this.cpu.Freeze()
this.memory.Load(0, this.hardDrive.Read(0,1023))
this.cpu.Jump(10)
this.cpu.Execute()
}

A.2. STRUCTURAL PATTERNS 113
Most clientswill use Start, but the low-level functionality of the sub-
systems is still available to those howneed it.
//the use of the facade
computer := NewComputer()
computer.Start()
//access to lower-level systems
subpart := new(CPU)
subpart.Execute()
Example - Fac¸ade as package Instead of encapsulating the fac¸ade in its
own type, packages could be used. We use the same example as above.
The package computer imports the package system containing the
subtypes CPU, Memory and HardDrive (declarations see example above).
The packagemaintains references to the subsystems in package local static
variables. The initmethod instantiates the subsystems as did NewCompu-
ter fromabove does.
package computer
import "system"
var cpu *system.CPU
var memory *system.Memory
var hardDrive *system.HardDrive
func init() {
cpu = new(system.CPU)
memory = new(system.Memory)

hardDrive = new(system.HardDrive)
}
Start is nowa function using the local variables to call the subsystems.
func Start() {
cpu.Freeze()
memory.Load(0, hardDrive.Read(0,1023))
cpu.Jump(10)
cpu.Execute()
}

114 APPENDIX A. DESIGN PATTERN CATALOGUE
Clients import the package computer (the package representing the
fac¸ade) and call Start directly instead of instantiating a fac¸ade object.
import "computer"
...
computer.Start()
Discussion Using a package does not require to create an Fac¸ade object.
The package and its subsystems are initialized in an init function, which
is called on import. The initialization could be done in a separatemethod
(initwould call thatmethod too), thus allowing clients to re-initialize the
package.
The Fac¸ade pattern should not be implemented with embedding. Com-
position should be used instead, because the purpose is to hide low-level
functionality and to provide a simpler interface. A Fac¸ade that embeds sub-
types publishes the entire public interface of the subsystem, counteracting
the pattern’s intent.

A.2. STRUCTURAL PATTERNS 115

### A.2.6 Flyweight

Intent Minimizememory use by sharing asmuch data as possiblewith
other similar objects; it is a way to use objects in large numbers when
a simple repeated representation would use an unacceptable amount of
memory.
Context Consider drawing a large number of coloured lines. A line has
an intrinsic state: its color and an extrinsic state: its start and endpoints.
Creating an object for each individual linewould not be feasible.
The solution is to reuse line objects with the same intrinsic state and
provide themwith extrinsic state to redrawdrawthem. Thiswill reduce
the number of line object to the number of colours. One line per colour, not
more. When a line should drawitself the extrinsic state, the start and end
coordinates, is passedwith themessage call. A small number of line stores
aminimumamount of data, hence flyweight.

Example The application implemented in this example draws a large
number of lines. The start and end coordinates of the line are generated
randomly. The color of each line is also selected randomly. There will at
most one line object per color. The drawing of lines will be simulated by
printing amessage on the console.
The first two listings showsome helper codemaking the following code
more concise.
We import the package fmt for console output to simulate drawing, and
the package rand for generation of pseudo-randomnumbers. We provide
the alias random instead of the somewhat cryptic package name rand.
package line
import (
"fmt"
random "rand"
)

116 APPENDIX A. DESIGN PATTERN CATALOGUE
colors is local static array containing the available colors. Having a
separate Color type improves type safety and readability. For simplicity
we define the type Color as an extension of string allowing us to treat
the two nearly interchangeably (as seen in the initialization of colors).
The function GetRandomColor selects a randomcolor fromthe colors
array.
type Color string
var colors = []Color{"red", "blue", "green", "yellow"}
func RandomColor() Color {
return colors[random.Intn(len(colors))]
}
The following listings are the core of the pattern implementation.
Objects of type line are the flyweights. A line’s color is its intrinsic
state, the state that distinguishes it from other lines. Note that line is
defined with package scope to avoid uncontrolled instantiation of line
objects.
type line struct {
color Color
}

Themethod Draw prints out amessage describing the line to simulate
real drawing. The start and end coordinates are a line’s extrinsic state, since
it is not stored in the line objects, but provided externally. Draw is a public
method. Even though clients outside the current package can’t refer to the
type line, they can access its publicmethods.
func (this *line) Draw(x1, y1, x2, y2 int) {
fmt.Printf("drawing %v line from %v:%v to %v:%v\n", this.color, x1, y1, x2, y2)
}
A line can be drawnmultiple times on different locations, but there will
only be one Line object per color. The the next listings shows howto ensure
that. Lines are not aware of each other, therefore lines should not to be
instantiated directly. The current package keeps the lines in amap. A line’s
color acts as the key.

A.2. STRUCTURAL PATTERNS 117
type lineMap map[color.Color]line
var lines lineMap
The init function uses make, a function built into GO, for allocating
memory for themap of lines. init is executed on package initialization.
func init() {
lines = make(lineMap)
}
GetLine is the heart of the pattern. GetLine checks the linesmap.
If there is already a line of the particular color the line will be returned,
otherwise a newlinewith the given color is created, added to themap of
lines and returned. This ensures that therewill only be one line per color
as long as themethod GetLine is used.
func GetLine(color Color) *line {
currentLine, isPresent := lines[color]
if !isPresent {
currentLine = line{color}
fmt.Printf("new %v line\n", currentLine.color)
lines[color] = currentLine
}
return &currentLine
}
The next listing shows how clients can create and draw a number of

flyweight lines of randomcolor at randompositions.
import "line"
...
numberOfLines := 5
for i := 0; i < numberOfLines; i++ {
aLine := line.GetLine(line.GetRandomColor())
x1, y1, x2, y2 := random.Int(), random.Int(), random.Int(), random.Int()
aLine.Draw(x1, y1, x2, y2)
}
Discussion In other programming languages a flyweight factory is nec-
essary to control the instantiation of flyweight objects. In GO we can
take advantage of packages as encapsulation units. To enforce sharing of

118 APPENDIX A. DESIGN PATTERN CATALOGUE
flyweights their type should be of package scope to avoid uncontrolled
instantiation. The flyweight objects are kept in a package variable.

A.2. STRUCTURAL PATTERNS 119

### A.2.7 Proxy

Intent Represent an object that is complex or time consuming to create
with a simpler one.
Context Consider a word processing application that allows images to
be embedded in the documents. Those documents can be big and contain
many images. Loading every image when the document is loaded takes a
long time and the imagesmight never be displayed.
The Proxy design pattern is a solution.

![](data:image/png;base64,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)

The Proxy (proxy image), or surrogate, instantiates the Real Subject (real
image) the first time the Client makes a request of the proxy. The Proxy
remembers the identity of the Real Subject, and forwards the instigating
request to this Real Subject. Then all subsequent requests are simply for-
warded directly to the encapsulated Real Subject. Proxy and Subject both
implement the interface Subject, so that Clients can treat Proxy and Real
Subject interchangeable.
Design Patterns lists four types of proxy. We implement a virtual proxy
(lazy initialisation) in the following example.
Example Images can take a long time to load and should only be loaded
when they are needed. An image proxy loads the real image only on the

120 APPENDIX A. DESIGN PATTERN CATALOGUE
first request. Subsequent calls to the proxy are forwarded to the real image.
In this examplewe showhowto provide a image proxy for real images.
Image is the Subject interface for real images and for their surrogate,
the image proxy.
type Image interface {
DisplayImage()
}
NewRealImage instantiates a RealImage object and loads the image
fromthe disc,which can take some time for larger images.
type RealImage struct {
filename string
}
func NewRealImage(filename string) *RealImage {
image := &RealImage{filename:filename}
image.loadImageFromDisk()
return image
}
loadImageFromDisk would read the image data. DisplayImage is
rendering the image on the screen. For simplicity,we simulate those actions
by printing out a statusmessage on the console.
func (image *RealImage) loadImageFromDisk() {
fmt.Println("Load from disk: ", image.filename)
}

func (image *RealImage) DisplayImage() {
fmt.Println("Displaying ", image.filename)
}
ProxyImage is the surrogare for real images. Like RealImage, Proxy-
Image keeps the filename of the image, and in a addition a reference to
an Image object. On creation, the proxy only stores the filename. This
is fast and lightweight. NewProxyImage does not load the image on
instantiation, but onlywhen the image is to be displayed.
type ProxyImage struct {
filename string
image Image
}

A.2. STRUCTURAL PATTERNS 121
func NewProxyImage(filename string) *ProxyImage {
return &ProxyImage{filename:filename}
}
The first time DisplayImage is called, a RealImage is instantiated,
which loads the file fromdisk. On subsequent calls, the request to display
the image is forwarded to the RealImage object.
func (this *ProxyImage) DisplayImage() {
if this.image == nil {
this.image = NewRealImage(this.filename)
}
this.image.DisplayImage()
}
This demonstrates howclientswould be using the real image and the
proxy. The instantiation of eagerImage will call loadFileFormDisk
immediately, even though it might never be displayed. lazyImage is a
ProxyImage and on instantiation only the image’s filenamewill be stored.
var eagerImage, lazyImage Image
eagerImage = NewRealImage("realImage") // loadFileFromDisk() immediately
lazyImage = NewProxyImage("proxyImage") // loadFileFromDisk() deferred

// file is already loaded and display gets called directly
eagerImage.DisplayImage()
// load file from disk
// and then forward display call to the real image
lazyImage.DisplayImage()
The call eagerImage.DisplayImage renders the image immediately.
Calling the same method on lazyImage will first load the image and
then display it. Loading the image will only happen on the first call.
Subsequent calls to the proxy image will be equally fast, since the call
is directly forwarded to the loaded image.
Discussion Proxy can be implemented in GO with object composition as
in other programming languages. In GO we have the option to use embed-
ding. ProxyImage could embed RealImage instead. Then ProxyImage
would gain all of RealImagemembers and ProxyImage could override

122 APPENDIX A. DESIGN PATTERN CATALOGUE
the necessary methods (like DisplayImage). The advantage of embed-
ding is thatmessage sends are forwarded automatically, no bookkeeping
is required. The embedded type is actually an instance of the type. This
instance needs to initialized to avoid null pointer errors. DisplayImage
would initialize the embedded type if necessary. The call is then delegated
to the embedded type:
func (this *ProxyImage) DisplayImage() {
if this.RealImage == nil {
this.RealImage = NewRealImage(this.filename)
}
this.RealImage.DisplayImage()
}

A.3. BEHAVIORAL PATTERNS 123

### A.3 Behavioral Patterns

Behavioural patterns identify common communication patterns between
objects and describe ways to increase flexibility in carrying out this com-
munication.

### A.3.1 Chain of Responsibility

Intent Define a linked list of handlers, each of which is able to process
requests. Loose coupling is promoted by allowing a series of handlers to be
created in a linked list or chain. When a request is submitted to the chain,
it is passed to the first handler in the list that is able to process it.
Context Consider a software controlling a vendingmachinewith a coin
slot. Themachine has toweight andmeasure the coin. Themachine should
accept different denominations of coins. Should the size or weight of coins
be changed it should be easy to change the software too.
The solution is to create a chain of handlers, one for each coin denomi-
nation.

![](data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAXAAAADcCAIAAADm0m1JAAAyFElEQVR4nOydeVwT59b4J4QQSFKSsEa4slkUpSIIijtucLFisSJY63Xh1tpq3VF7rTt668cFl2pbrHqtvb0udQGXilq3gtVbBMG6UBdUhLDJEiAIIcD8Ph/O+85v3mQyBAkk4Pn+lZyceZ4zk5OT8zzzzHnMSZIkEKQ5kpOTg4KCjG2F8REIBNXV1ca2wnQxN7YBSEeC04SxrTAajY2NxjbB1MGAgrSAoKCgq1evGtsK46BSqSwtLY1thaljZmwDEATpPGBAQRDEYGBAQRDEYGBAQRDEYGBAQRDEYGBAQRDEYGBAQRDEYGBAQVoAruxC2MGAgrSAsrIyY5uAmDQYUBC9yM3NJQiiuLjY2IYgJg0GFEQvrly5QhBEaWnpq1evjG0LYrpgQEH0AgJKQ0PD2bNnjW0LYrpgQEGa5+7du8+fP4fXR44cMbY5iOmCAQVpHggiERERXC43KSmpsrLS2BYhJgoGFKR5jh49ShDEggULhg8fXltbm5iYaGyLEBMFAwrSDLdu3crOzv7LX/4yePDgSZMmUfEFQbTBgII0A4SPyMhIMzOziIgIHo/3yy+/lJaWGtsuxBTBgIKw0djY+NNPPxEE8cEHHxAEYWNjExwcrFarT548aWzTEFMEAwrCxo0bN3Jzcz08PPr16wcSiCw46kEYwYCCsAGBY9KkSVRt6vDwcEtLy2vXrhUWFhrbOsTkwICC6KShoeHYsWMQUCihtbX1u+++29DQcPz4caNah5giGFAQnVy7dq2oqKhnz559+vShyyG+4Ao3RBsMKIhOIGTQ0xMgLCxMJBLduHHjxYsXRjINMVEwoCDM1NfXw6AmKipK4yOBQBAWFkaSJI56EA0woCDMmJub29raEgRRUlKi/enLly8JgrCzszOGaYjpggEF0YmuuZKioqJr165ZWlqOHz/eSKYhJgoGFEQnEFCOHz/e0NBAlx87dqyhoWHMmDHW1tbGsw4xRTCgIDrx8fHp1atXcXGxxn7G1OIU45mGmCgYUBA2tEc9ubm5N27cEAqFYWFhRjUNMUUwoCBsQEBJSEioq6sDyU8//dTY2Dhu3DihUGhs6xCTAwMKwkaPHj38/PzKysouXrwIEhjvwBM9CKIBBhSkGeg1ULKzs2/duiUWi0NDQ41tF2KKYEBBmgGeDDx16lRNTQ2ElfHjx/P5fGPbhZgiGFCQZnBzcwsMDKyqqjp37hzMzuJ4B9EFBhSkeSCCbN68+e7du3Z2dqNGjTK2RYiJggEFaR6o/5iamgq173k8nrEtQkwUDChI8zg5OQ0bNgxe43o2hAVzYxvwRnD48OGOviuwg4MDVFfKzMz8448/jG1Oq3jvvffc3d2NbUXnBANKe3D//v0lS5YY24pWMXbs2JMnT06ePDk6OtrYtrSKzMzM+/fvY0BpIzCgtAfm5uYSicTYVrQKiUQyatSoadOmdfQTeeutt5RKpbGt6LTgHAqiL4sXLx44cKCxrUBMGsxQEH0JCQkxtgmIqYMZCoIgBgMDCoIgBgMDyhtBRkaGSCTSKLzWDqSlpXE4nPr6epVKJRKJBAIBvG1nM5B2w3zjxo3GtuE16dev3+jRo41thRE4ePDgli1bnj9/LhAIoqOjN23a1Owhfn5+zd7aSEtL69evn1qtNjc3137bSvh8vlKphDZb3xpispgHBARQiyA7Fhs3bnwDA8r+/fvXr19//PjxgICA4uLiy5cvG9siBPn/mPF4PH7HxNiXrk24ffv2nDlztm/frku+du3azZs3BwQEwOrVyZMng0JOTg5Ujba1tf3oo4+qq6upYyUSCX2sAcOQHTt2yGQye3v7xMTEZq2Kj4/39vYWiURSqTQsLOzJkycsjeTl5YWGhopEIjc3t4SEBPaWq6qqZs6caWdnJxaLp06dSpkN7ScmJnp6eopEosjISJAHBgbu27dPI9tiFCJGAedQTILKysr4+Hh/f/+JEyc6ODhQvx8Nef/+/fPy8oYPH67dwqRJk7p06VJcXPzw4cOsrKxly5ZRHykUiuTkZA19pVKZn58/c+bMpUuXNmueUCg8ePBgZWWlXC6XSqXU4zyMjUyePNnJyamkpCQtLU2jurU2M2bMyMnJefLkSX5+vkKhoJsNjyzcvHmzsrLy888/B8ny5cvPnj3r6uo6a9asW7dusQgR43D16lWyY7JmzRpjm6Av7KZGR0fLZLLo6Ohr1641NjayyDMyMgiCUKlUGi3k5eURBPHs2TN4e/LkSXt7e7oC/MzUajX1uqioiCTJ69evczgcaBzk4v9FJBJRh9BJSUnhcrm6GpHL5RqW0Buhm0GSJDzflJ6eDm8vXLhgZ2dH13zw4AHjFXv58uWOHTv8/Pz69Olz+fJlFqE2aWlpZ86cYfk6dFFbWwu7Jr7GsW8OmKEYn6ysLGtray8vL09PTw6HwyKHZe8KhUKjBdjHz8nJCd5CgsDeqY2NDcyVkiRJv/tTUlKiaIKeXCQkJAwcONDW1lYikYwZM6ahCcZGIEbQLWGxIT8/nyCIkSNHSpqIjIxUKpWNjY2UQrdu3RgPlEqlXk3k5+dTZ8ooRNoZDCjG5+bNmydOnJDL5b6+vqGhoYcOHaqpqWGUOzo6Ojs7a49f7O3tqd8nvDDgJqGFhYWRkZGLFi0qLCxUKBSnT58mCIIkSUZlR0dHuiWQsFBwuVyCIKj41aVLF4IgHj9+DCGsoqKipqbGzOz/+yT9NXD//v1ly5a5uLhs2LAhNDQ0JycnKiqKUWio00daBAYUk+Cdd97ZuXNnbm7utGnT9u7du2XLFl3yVatWLVu27Pbt25BNQE1GZ2fnfv36xcbGqlSq0tLSLVu2REREGMq2mpqahoYGiUTC4/HkcvmGDRtYlLt06RIUFERZsnXrVvqnHh4eFhYWV65cgbcODg7h4eExMTHl5eUEQTx9+vTs2bPsxowYMaKxsfHy5cspKSnTpk2zsrLSJUSMg645lPLy8n/+85+BgYESicTc3NzBwSEkJGTPnj3wKRzr6OjI+NZQbNmyZU0TjJ+2/xxKbm7u6x3YUlO1Z0no8n379vXs2VMoFNra2sbExMBHT58+DQkJgRsx06dPr6qqAnlcXJxQKITfmLCJ9evXa8ynaL/WeBsXFyeTyUQikb+//65duyB70nWgXC6Huzzu7u4rV67UmIj59ttvpVKpUCiMi4sjSbKiomLmzJn29vZCodDT03PHjh3avTd7ZXRdLkZwDqVNYQ4ot27d0jX6/Z/D2iWgQP5MdapB+weUUaNGOTs7z5kz5+LFi3V1dfof2IHmjzs9GFDaFIYhT3Fx8dixY2EYPH/+/Ozs7Lq6ury8vAMHDnh7e7NEmcLCwlYnTKZLQ0NDamqqXC7/5ptvQkJCHBwcpkyZcuzYsaqqKmObhiCmAkNAiYuLg7n6efPm7dy508PDg8fjOTs7z5gxIzMzk7EVThMymYwuPHfuXEhIiI2NjYWFhZub2/z588vKyjT0b926NXLkSCsrK3t7+6VLl9JvN3A4nKKiIro+/Q5I+8PlcisrKzMyMtasWePr66tQKA4dOhQVFWVvbz927Ni9e/d27niKIHqhPeTp1asXfJSTk6MrsQEFliHP5s2btfvq1q3by5cvKX0+n68xf7Zt2zaNLjSg22DcccTTp0+3b98eFBQEdy7glsSgQYM2b9788OFDDWUc8pgOOORpUxie+3r27BkscHJxcXm9IPXixYsVK1YQBBEaGrpnzx5HR8fExMQPPvggOzv7yy+/3LZtG6ipVKpPPvkkLi7u2LFjUKn0yJEjixYtomKHTCaDJIUxvhQWFp4/f15b7u/vD7dRNUhNTaVSJDoDBw4Ui8Xa8t9++41xODNs2DCBQODu7r6wiZKSkrNnzyYmJp4/f/5GE8uWLevVq9f4JgICAoybWCFIu6KdoUDWIBaLWeIQHKsrQ/nuu+90deft7U3pm5mZKRQKkiSppzCcnZ3pvbBPyvbs2ZOxi3PnzjHqM65YJwjiv//9L6N+7969GfUfP37MqA/T2Bp71tjZ2c2dO3fFihUsFxNpTzBDaVMYMhR3d/cHDx5UVFTk5uZ27dpVV2hggWXLiNLSUuq1ra0tpAZCoRAkLaqU0aVLF8ba5YzpCTxCJhAItOW6qi4PHTqU8fS1G1Gr1cnJydbW1uXl5bAmDRZuhoWFhYeH//Wvf9VYjoEgnRbtDIV6EGvBggUaH1HrAkCh2Qxl06ZNGi3AEx8a+rpuPFOzvIyx0OgTE0ql8vjx41OnToUV6ICLi8u8efMuXbpEv6/cDqauWbMGDKD3Rc0Ta6R+hmX//v3Qy86dO0FCLV1bunRp2/X7emCG0qYwZCiLFy8+cOBAcXHxzp07zczM5s2b17Vr1+Li4osXL27ZsuX+/fvNBqnQ0FAej6dWq7du3erv7z906NCampr09PT//Oc/Xl5e+jzeClCZS0ZGhp+fn55HtTXFxcVnzpw5derUpUuXqHykd+/e4eHh48eP79u3r1EmTeC5QYIg+vTpQwnv3LkDL3x9fduu6/T0dHjRt29fXRLkDYEhoDg4OPz888/h4eH5+fnbm2hpo127dt24ceOSJUtevnypUQMJlmnqyZAhQ7Kzs+l+qevuTzugVqt37dqVmJj422+/wQNsXC536NChEEd0PcbWblB39H18fChhewYUMzMzqhd4MgADyhsI87M8AQEB9+7d27BhQ//+/a2trblcrr29PSy917PdmJiY8+fPjxkzxtbWlsvlisXiwMDA5cuXT5kyRX/jtmzZEhkZKZVKTeFGCY/H2717d0pKioWFxbhx4/bt25efn5+cnBwTE2P0aFJeXv7ixQuCIEQikYeHByWn9gylfupffPGFn5+fjY2Nubm5lZWVt7f3pk2b6A/4ymQyDofTtWvXX3/9dfTo0UKh0MbGBlbQA9XV1QsXLnR0dLSysgoODs7Ozoaw1b17d6h4QAUUa2trT09PkJw5cyYsLMze3t7CwsLDw2PNmjUqlUqjUxcXl8uXLwcFBVGZKdLxwHoo+nPw4MHjx4/X1ta29MC2NpUqBOnp6ZlEg/o9w50ptVptaWmp7QPUEzTUU8IikcjMzIxeTfbXX38lSbKqqsrf359+LPWIxocffgiNVFVVwR/AsGHDSJKsr69n3L108uTJGp3Cfw+8brtrhXMobQo+bdwCpk2bFhERYYLVJ6nxzuPHj8fQePz4MWy+CTmUUqk8evToixcv6urqVCrVL7/8AkdRc6hUO2ZmZjCvTNWOe/ToEUyywgBn9uzZZWVld+7cefXqFShQo5uMjAwYmYJkw4YNBw4c4PP533//fUVFhUKhCA4OhjVHcDeQ6rSysnLPnj3V1dVUiEE6HBhQOgO6HokAfHx8IGWora1NTk4eM2aMRCLh8/nww6bfCKdmdlesWDFixAgOh0MNoJycnEpLS//1r38RBNGzZ8/du3dLpVIfH58BAwaAApW5UBMo/v7+5eXlsGZapVLNmDFDLBZLJBIIZCRJQhkkqtNly5Z99NFHAoEA6qQgHRHcirQzQP0mMzMzqbs88fHxs2fPpiZQXrx4ERgYyPjAEbWEjwpM77//Pry4e/cuvPD19b1+/XpdXR1BEOPGjaNKH0EpEw6HQ92Go9/i+fXXX6kURgMul+vq6krvdNq0aYa4GM1z8+ZN+gyOnqjV6rYxp1OBAaXDo1Kp/vzzT4IgLCwsqOew6FEGAkpcXBxEk9jY2Pnz54vF4nnz5u3evVtjtAKTqW+//TZd4ujo6OTkRAUj6kkFuVwO4cPDw4MSQoYiEAi8vLyuX78Owt27d3/22Wd0s0mShLwJuhCLxdSMT1uzdetWiIyIwcGA0uG5d+8erDB+55136Av/NQIKzKfAxC2Pxzt69OjevXtBAslFVVUV3KT39fWFn3pxcXFBQQGlQC1BPn369Mcff/zq1atp06ZB19R459WrVxDdfH19zczM3NzcQP6vf/0rNDTUzc2tsLAwJSXlhx9+WLNmTWBgINVpnz592u1e3qBBgxwcHF7vWAsLC0Ob06nAgNLhoQIHfe1ffX09jFbMzc3feecdeIoqKSkJbq/Ak9+Qwzs5OcEzU3fu3IHJVKodqmVIYUJCQhwdHYuKin7//Xf4QVJLhKkc586dO1CDAiSjRo3q379/amrq7du3qawHJn2PHTvG2Gk7EBMTExYW1m7dvVHgpGyHh5qDoK9e+/PPP+E2Z48ePeBW8apVq6KiogQCgUQimTZt2vHjx2H5CfVL1m5HI1SJRKKkpKTBgwdbWFg4OTmtXr16+vTpoEAFFI0lbVwu9+LFi4sXL4aqOnw+38PDY9KkSYcOHYLFJozGIx0XzqxZszropDqPx4MiCabP2iaMbQVCwJxxQUEBZihthLn+i18RBEHYwSEPgiAGAwMKgiAGAwMKgiAGAwMKgiAGAwMKgiAGAwMKgiAGA1fKtgdWVla4DsVEoNdkQAwOBpT2oKamBgOKiZCeni6Xy02nRHEnA4c8CIIYDAwoCIIYDAwoCIIYDAwonRO1Ws3n8319fb///nvO//LTTz8Z2y6Dce/ePbFYfPToUUrS0NBgb29PL9CPtD8YUEyCn376icPhxMbGQoFCDofz97//vTUNPnz4sK6uztfX18vLa/v27bAPdEBAQLMHJicnQ/ThcrkuLi7UzvbG4m9/+xuHw7l37x5dqFQqJ0yYEBISMmnSJErI5XJHjhx58OBBI27ehGBAMQIymWzhwoV0SVpaGvWDh6KK+vz4WYDqSr6+vgMGDFi4cOGrV69sbGzoW/boAiyZOnXqunXrlEplTEwM1Ls3FpmZmZaWll5eXnTh6tWrc3JytmzZoqHs4+OTl5dH1aZD2h8MKCYBBBEopAg1igICAvLz899//30HBwdzc3MPDw+qYuOBAwc4HM7kyZO9vb2FQuG6detAXlJSMn78eKFQGBwcnJqaSlUtKikpycnJ0dhPR61Wr1692sXFhc/nDx48mEoBwJKYmJiVK1eOGjUKdrfQpVxaWjphwgShUBgSEjJ//nwOh3P+/HmCILZt28bhcA4ePEgQxKRJkzgczsOHD3U1Ul9fv3z58r/85S/m5uYymeyLL74AOVSivH//fm1tLY/Ho/IUuVz+9ddfT548maovSWFnZ0cQxLNnz9ryu0JYMfbGQG8EsNFXSUnJsybs7Oyio6PhdU1NTWNjo0Qi4fF4nzXh5OTE4/FqamqOHTvWu3fvL774YtWqVSKRiMvlVlVVkSQ5d+5c2EZr165dHA5HKpVCL8OHD4cdc8LDw6EeWnl5OUmSUPnxH//4B92kDz/8EKrbr1u3zsbGxtXVtb6+niTJ7t2783i8R48enT59Grb+q6ur06UMEYfeY0FBAdU4lHd8++23RSJRY2OjrkZgVDVhwoQtW7Z8/PHHy5cvBwu/+eabOXPmwJnClriwV//GjRsJgrh8+bL2dYZtc5OSkli+i9fe6AvRBwwo7QEElE8++UQ7oF+9elU7Rff19SVJsry8vKGhoaCg4Pnz50FBQWZmZjU1NSRJDho0iCCI58+f19fX83i8bt26kST522+/Udv3wX43rq6u0DtMzRw/fpyyhxphQVCbOXMmQRBZWVkVFRX0StHTp08vKCjQpfz7779r9Ojo6Ajtd+/enc/n19XVVVZWcjicIUOG6GqEJMnly5cTBLF//37tLRm//vprgiC++eYbunD06NE8Ho9x/0bYNuThw4cs3wUGlDYFV8q2H7Nnzw4NDSUIIjo6esiQIR999BGUqr906RJBEOvWrVu9evXNmzcHDRrk7++vVCqXLFly5MiR6upqONzLy8vS0rKxsfHOnTuenp6urq4PHjxQq9VQvRV2I50wYQJs6EWv0kqfoAFgq8C0tDR3d3dKaGlpefv2bZIkIyIiRo8ePW/evKysLJlM9u9//5tROSEhgbHHysrKx48f+/n58Xi8//73vyRJ9u3bV1ePBEEsXLjw6dOn8+fP/+yzz6ZPn757925qC1QoaqsxnfTs2TNXV1fG/RuvXLnStWvX7t27G+5LQ1oGBpT2o08TBEF8+umn3bp1Gz9+PMjps7DU69jY2P3790dFRYWHhxcXFy9atAgCx6NHj6qrq2HlOMy2wORIUVERtQfg8ePH6QElPT3dzs4OdtUClEollK2miksTBOHi4nLixAmCIMLDw6dOnZqUlHT69OmcnBxdyrp6zMzMJEkSSu1DmPPz84N5De1GSJJ0cHA4cuSISqVaunTprl27wsPDx4wZAwoZGRnm5uY+Pj4aV5Jxh+bU1NSHDx9SM0qIUcCAYny0b/H4+/tfvXoVCnE/fvz4+++/p+rIQxDRDigQL9atW3f9+vUdO3ZQP++Ghob8/HyRSLR582YLC4uQkJBevXoNGTIEttexsrIiSfKPP/6IiIgwMzMDS6DxMWPGnD59+tSpU7qUoce1a9f+9ttvO3fupHqEvQQzMzP37t0LlvTt29fZ2ZmxkZSUlM8//zw4ONjKyurSpUscDgc0gRcvXsAWZQKBYMaMGRKJBM70wYMH2pdx48aNDg4OCxYsaN9vD/m/GHvM9UYAcyiMNDY2Wltbd+3aFd727t0bJghSU1N79uzJ5/NHjRoFw4orV66QJLlkyRJq3hFmYUtLS2HCZejQoXw+f+LEiSB/9uwZtLlw4UJra2v4uq9duwbCnTt3uru7c7lca2vrkSNH5uTkkCTp6elpaWkJc5+QU4waNUqXckVFRVBQEJ/Pj4qKCgoKouZEFApFQECApaVlVFSUh4cHn8+HBhkbuX79uq+vr6WlJY/H6927948//ki/OGvXrpVKpbC7EDVpAlNCxcXFdE1IhU6cONHsd4FzKG0KBpT2gCWgdA66du0qEAgaGhraoa/nz59zuVx66FEoFG5ubsuWLdPncAwobQoOeZDWUllZmZubGxgYSO2g3qbA/Wa6RCwW49oTEwEXtiGtBdab4dZ/CE7KIgZg0KBB+PgMAmCGgiCIwcAMpT2ora3tBCUg8/PznZycjG1Fa6mqqoK7ZkhbwMFkFdETW1vb9evXw/M1CMIIDnkQvYiPjy8rK1u1apWxDUFMGgwoiF6sWLGCIIiysrJvvvnG2LYgpgsGFKR5ID2B15ikICxgQEGaB9KTmJgYqVSKSQrCAgYUpBkgPZFKpatWrVq0aBEmKQgLGFCQZoD0ZOHChWKxeP78+ZikICxgQEHYoNITKAsgFosxSUFYwICCsEFPT0CCSQrCAgYURCca6QmASQrCAgYURCfa6QmASQqiCwwoCDOM6QmASQqiCwwoCDO60hMAkxSEEQwoCAMs6QmASQrCCAYUhAH29ATAJAXRBgMKokmz6QmASQqiDRZYQjSB9GTQoEGwNxAL3bp1MzMzgyQF66QgWGAJ0SQ+Ph52CG4RNjY2paWlbWMR0pHADAX5P5w/f56xzmN1dXVFRYWZmZlMJmM8MDs7u1u3bm1vIGLSYIaC6MWuXbvmz58vkUhgp1EEYQQnZREEMRgYUBAEMRgYUBAEMRgYUBAEMRh4l0cnV69ePXbsGEEQb7311qZNm7QVysrKVq5cqS2XyWSrV6/Wlufm5m7cuFFb3q1bt5iYGG35n3/++dVXX2nLfXx8Pv30U215enr6/v37teUDBw6cOnWqtjw5OfnIkSPa8tGjR7doKyyVSgUr3DRo6XVzdHRcs2aNtlzXdfPw8FiyZIm2/OHDhzt37tSW9+7dm/GO+O3bt/ft26ctHzBgwLRp07TlKSkphw8f1pZ/9NFH/v7+2vI3CxLRAeWUDg4OjArPnz9nvKQ9e/Zk1E9PT2fUHzx4MKP+xYsXGfXfe+89Rn3G6EAQxIwZMxj1v/76a0b9mJgYbeWTJ0/Cb1L7o6qqKsZ27O3tGfvNyclh1O/Rowej/u3btxn1YU9lbS5dusSoP27cOEb9o0ePMupPnz6dUf/bb79l1D9y5Aij/hsFZijNMGrUKMa/KVjNFR8fry2XSCSM+i4uLoz6ulZ29OzZk1Hf1dWVUT8gIIBRv0ePHoz6QUFBjPo+Pj7awgEDBqxYsSIwMFD7Iz6fz9iOpaUlY78tvW5du3Zl1Hd0dGTU9/LyatF18/f3Z9Tv3r07o/6wYcM09LOzs0tKStzd3Rn13yhwHYpOvvrqqwULFixcuHD79u3GtgVBOgY4KYsgiMHAgIIgiMHAgIIgiMHAORSdNDTBbcLYtiBIxwADCoIgBgOHPAiCGAwMKAjSWn755Zevv/76yZMnxjbE+GBAQZDWcuDAgblz5+paCf1GgQEFQRCDgQEFQRCDgQFFJyRJNjQ0NDY2GtsQBOkwYEDRya5du8zNzRkLCyAIwggGFARBDAYGFARBDAbWQ0GQ1hIWFubs7Ozl5WVsQ4wPBhQEaS0fNmFsK0wCHPIgCGIwMKAgCGIwMKDohMfjvfXWW7oKoyIIog2WL0AQxGBghoIgiMHAgIIgiMHAgIIgrSUhIWH9+vX37983tiHGR985lDNnziQlJTk4OLS9SYgxqaurc3Jymjt3rv6HPH369B//+EevXr3a0i6TprCwUKlUymQykUhkbFuMxpMnT3788Ud9F7Y1NDTMmjXL19e3ja1CjExFRcWBAwdadAhJkuPGjWPcPhl5c1i7di0OeRAEMSQYUBAEMRgYUBAEMRgYUBAEMRimElAyMjJEIlFDQ4OxDTEaaWlpHA6nvr5epVKJRCKBQABvjW0XYiqAh1RXV5uye7RfQIHLIaJx69Yt6lM/Pz+lUvl6m37SW5ZKpePHj3/27JlBbWfukfo6Nd62Ej6fr1Qqk5OTDdJaRwe+Vvj9UJ7Tohba3z3a1ENM3D3aO0NRKBTK/6Vfv34Gb/nx48dWVlaRkZEGbBkxIuAq8PuhnOc12kH3aB8MFlBu3749Z86c7du36ymnI5FINFK49PR0a2vr2tpaeJuYmNitWzeCIKqqqmbOnGlnZycWi6dOnVpdXa3RlJ2d3WeffZaRkQFvtfXhv2LHjh0ymcze3j4xMZE6VqlUzp4929HRUSQS+fv7Z2Vl6WqE5Vzi4+O9vb3hzzAsLAx2k9PVaV5eXmhoqEgkcnNzS0hIaPYiM1oCjScmJnp6eopEIurXEhgYuG/fPu2fny65UXhtt2G5FIxfLtCse7A00v7u0VIP0WUJo4e0kXu0NqBUVlbGx8f7+/tPnDjRwcGB8mZdckYUCoVGCufv7+/k5PTzzz/D28OHD0NFrBkzZuTk5Dx58iQ/P1+hUCxbtkyjqaKiol27dvn7+8NbXfpKpTI/P3/mzJlLly6ljp02bVp2dvadO3eUSuWePXuoDTSa7ZSOUCg8ePBgZWWlXC6XSqWTJk2iPtLudPLkyU5OTiUlJWlpaVevXm3uYrNZcvjw4Zs3b1ZWVn7++ecgWb58+dmzZ11dXWfNmkUfXeqStyetdxuWS8H45QJ6ugdjI+3vHi31EHZLNDykrdyD1I+EhISMjAwNYXR0tEwmi46OvnbtWmNjI7scjBP/L0OGDKE3BZ+q1WpKsnbt2okTJ5IkqVQqBQJBVlZWcXExJC+gcOHCBTs7O42Wu3Tp8sEHH+Tk5JAkyagPykVFRSRJXr9+ncPhgIVFRUUEQTx48EDjHFkaoc4FRvV044GUlBQul0tZqNGpXC4nCOLZs2egfPLkSXoj2heE/fS1LQdevny5Y8cOPz+/Pn36XL58uVm5QqHYvn07Y1O6ePLkyQ8//KC/fovcBtC4GuyXQuM6t8g9dDXSIvfQ9nZGD2F3D5IkWTxEf/dg95CWugcLa9asIUmyVTVls7KyrK2tvby8PD09ORxOs3KCIEpKSszN9ep0ypQpffr0USqVp0+f9mrizp07BEGMHDmSCoV1dXXUH4V2y/n5+br0bWxsYH4LdvMyNzcvKCggCMLDw0PDDJZGqB7T0tKo+aCEhITNmzc/evSogQZ8pNEpeICTkxN8Sr3QBYslBEHAkFAbqVQKV+/SpUslJSXNytuB13AbDdgvhcZ1BmGL3EO7kddwD0YP0d89zM3NW+QhuiwxM/ufUQijhxjcPVo15Ll58+aJEyfkcrmvr29oaOihQ4dqampY5C3i7bff9vb2PnXq1OHDh6dMmUIQRJcuXQiCePz4saKJioqKmpoa6npp0yJ9UH769GlrGiksLIyMjFy0aFFhYaFCoTh9+jR8tYzKjo6OlB8QBAF/RxRww4t+H53dEm2T7t+/v2zZMhcXlw0bNoSGhubk5ERFRbHI243Wu01LPaH1jbS/e7B7SEvdQ9tD2so99ElmdA15KGpra//zn/8MHz583bp1uuTaSRodxk+3b98+dOhQS0tLuVwOkvDw8KlTp5aVlZEkmZ2dfebMGV3H6tKnK2scGB4eHhISUlhYSJJkenr6/fv39WmE3g443IULF0iSzMvLg78LtVqtq9OgoKDo6Oja2tqSkpKBAwfS21QoFBYWFufOnWM/HZbTt7e3j4mJycrK0lNO9dvWQx4KfdwGJNrn2OyloF63yD10NdIi99A2GN4+evSoRe7B4iH6u4cuD3k992ABhjyGCSgUKpVKl1zX9xoXFycUCq2srGDKSigUJiUlwUcFBQVcLnfkyJGUckVFxcyZM+3t7YVCoaen544dO9gDirY+y5dXUVExa9YsUPbz86PGnOyNaLQTFxcHj7H7+/vv2rWL3WPkcjnM4bu7u69cuVLjLL799lupVCoUCuPi4l7j9Fm+C5ZvsD0DSrMmUXLtc2z2UugTUPRvpEXuoSugqNXqFrkHu4fo6R66POT13IMFCCj61kNJTEx0c3PD8gWdHihfsHDhQv0Pyc7OvnHjBpYveMNZ24SpLL1HEKQTgAEFQRCDgQEFQRCDgQEFQRCDgQEFQRCDgQEFQRCDYZiAolAovvzyywEDBkilUh6P5+jo+Ne//vW7774zSONtx9atW+Fel/6HcJqQyWR0oUwmA3kb2EiwdMRojEmBjtGhHePLL78MDg7u0qULj8eztrYePnw49byuTvRctcKysO3WrVu6njJ4vRUy7QYsbW6RnaDv6OjYynZeD42OGI1pJQZc2IaO0dEdg/G7O3r0KKMyLGxrbYZSXFw8duxYeNxg/vz52dnZdXV1eXl5Bw4c8Pb2bmXjrwd7QQpEgza6XOgYHZ3q6monJ6edO3fK5XKlUkmVxVi3bh3bYXrGKl0ZClVzYd68eRof0Zf6KhSK5cuX9+zZ09LS0srKqnfv3rGxsa9evaIHQkdHx9TU1BEjRlhaWtrZ2S1ZsqS+vp5qobS0NCYmpnv37nw+XyAQ9O3bFx5VoI799ddf+/XrZ2FhAZGSJMmff/45ODgYkm1XV9d58+aVlpayR99mD6S6o58p4x9Rbm5uRESEp6enQCDgcrk2NjbBwcH0hy/0Oetz58716tWLz+f369fv999/1+ePSJ+zZrxc1DdlkAwFHaMTOEZlZSWlUF9fLxQKCYKwsLBgdAPDPMtDbUAJNSYYKS4u9vT01P6SAgIClEoldTJ8Ph+e6KHYtm0btFBYWOju7q5x+P+cQBOWlpYCgYAu37x5s3aP3bp1e/nyJf0KavsN+4H6+w1jcRoOh3P+/Hm6ASxnnZaWxuPxKLm1tTWlSW+BboyeZ619uSgMFVDQMTqZY1RXV1tYWEANB8Zv0zABBc5ELBazHDt79mwwcfr06WVlZUVFRePGjQMJPE5KneEnn3yiVCqprTD79+8PLcyaNQskY8eOff78uUqlSklJOXv2LP3YsLCwgoKCysrK58+f5+TkwBWH569ra2uPHDkCaosWLaIM0/6+mz1Q+1uhQz/rwsLCpKSkwsLC+vr62traixcvgk5oaCgoNHvWEyZMAElsbGxNTU1sbKxGRxp+o89ZM14uutmGCijoGJ3MMVasWAEKGzZsYPw22y+gODs7QwWHiooKkEApTSj1SJ2MmZmZQqGAEm0gcXZ2Bn2o9WBmZkYFVArqQlAlDkiSZLmP4O3tTalp+02zB+rvN7W1tbGxsT4+PpAoUri5udEtZzlre3t7giB4PF5tbS00SP0v0Vug/Eafs2a8XHTaM6CgY9B1TNkxdu/eDbeQgoODdVUgMcykLCScFRUVubm5unSgfJ5UKrW2tgaJq6srvICaVICtra1YLIYiBiChala/fPkSWrCzs2PswtbWln5Dgd6sBqWlpSyno+eBujJbOvPmzVu9evUff/yhMReoUTSI5azLysqggjefz4ccGDRbabz25WoL0DE6jWNs37597ty5JEmOGDEiISGBveJiawMKlaPGxcVpfESdv4ODA0EQ5eXllZWVIMnJyaF/9D+m6K64RbWgqxqdxklSzW7atEkjjlL1r2DgythRswfqw9GjR8GwlJQUtVqt6xYDy1lDTUCFQqFSqQiCUKlUFRUVLD3qb7yeVThbAzqGLjqWY2zatGnx4sUEQbz77rvnzp3TyKoYYMxetNE15CkqKqLMXbRo0dOnT9VqtVwuP3DgQK9evUDn008/BYXo6Ojy8vLi4uLw8HCQrF27lnEOSUNCDZXfe++9Fy9eqFSqGzdu0IfKGv8ML168gCTQ3t7+0qVLKpVKoVBcvnz573//++bNmyk1qsrm7du39TyQsTvGuTe49DweLzMzU6FQzJkzR+PYZs86IiICJDBUXr9+vca3pqGvz1kz2k/HUEMedIxO4BjU7ExERERdXR27GxisYluz65eKiooYC+T6+flVVVXpcwWbnczX/oVs3bqV0aT169dTOtOnT9c2mP1Axu4Y/Qbq4FJQtzP09xuNyXyBQGBpaUnvSLuFZs9a1+WiaM+FbegYJu4YjIfoykIMWQKyrKxsw4YN/fv3t7a25nK59vb2ISEhe/bsoSt8/vnnXl5efD7f0tLynXfeWbt2bXV1tZ5XkFpu4OnpaWFhYWlp6ePjc/r0afZfyPnz58eMGWNra8vlcsVicWBg4PLly58+fUopFBcXR0ZGSqVSKsVt9kD9/aaysnL69OnW1tYikWjcuHFUMq+/31DLDSwsLPz9/VNSUvRZbsB+1iyXCzBsCUh0jA7tGK8RULAEJPJ/wBKQyOuBJSARBDEwGFAQBDEYGFAQBDEYGFAQBDEYGFAQBDEYGFAQBDEYGFAQBDEY+j7TUVNTk5yczPKgF9I5eNVEiw5Rq9WZmZkSiaTNjEI6AHl5eS0IKFZWViqVSuOBSKTzUVNTQ5XY0RO1Wl1XV4e+8YYDq4pb8NRpcHAwrpTt9MBK2RYdIhAI+vfvHxUV1WZGIR2ABw8e4BwKgiCGBAMKgiAGw6QDSlpaGofDqa6uFolEAoGAw+FQtXk6B3/729/27t1Ll3zyySfbtm0znkUdBvQN08RgAQW+YOpL1XjbGvh8vlKpTE5ObpElIpFIKBQOGTLk7t27rbehNei6FGlpaSkpKdHR0XThypUr//nPf7IX4OpwoG+w29OZfMOkM5TWoGjCx8dHo56N6fDVV19NnTpVo+he165dAwICDh48aDy7Oj/oG23H6wcUPf9n4uPjvb29RSKRVCoNCwuDsuZw7I4dO2Qymb29fWJiIqWfl5cXGhoqEonc3NwSEhLYG6+qqpo5c6adnZ1YLJ46dapGeU4ej/fBBx9kZWWxK1M9enh4rFq1Ck6Kfnb01yw9fvXVVy4uLkKhUCaTQYGsiooKkUg0bNgwKCksEomooodQI2f06NHaJxUcHHz27NnmLr9Jg77xxvpGm2coQqHw4MGDlZWVcrlcKpVOmjSJ+kipVObn58+cOXPp0qWUcPLkyU5OTiUlJWlpaVevXmVvfMaMGTk5OU+ePMnPz1coFNRudYBarU5ISBg0aBC78ocffgg9pqamXrp06fV6zMnJWbBgweHDh6urq+/duzd48GDYRIJKyBUKhVKpjI+PB/3c3NzS0lJqNyw6vXv3vn37NrsZnQP0jU7oG4zV3LShl4AUNyESieC6ALAfGvUWPtXYwiMlJYXL5VKbpxUVFZEkef36dQ6H09jYSJKkXC4nCOLZs2egf/LkSaoROITeIGwOkJ6eDm8vXLhgZ2dHaYrFYnNz8/79+5eVlbEoQ9VvqscTJ05AL/TuqNe6GgHLuVzu3r176bs3AtqWkySZmZkJq8i0L/WNGzfMzMz0/F4MTitLQKJvaDTSmXyDndfflweGoPAXUVJSAm/hI+ot9QeSkJAwcOBAW1tbiUQyZsyYhibgI9gNgM/nkyQJQvhWqMrG7HvHwPc9cuRISRORkZFKpbKxsZGypKCg4NWrVz///DOLMmwNQ3UEW0+9Ro9OTk5Hjx49duyYq6tr7969T58+zX4NpVIp5L3aH1VUVHTcZezoG2+4b7TtkKewsDAyMnLRokWFhYUKhQIuJUsVW6i1S+0VAn9KAJfLJQiCcjiCIGDXuMePH4ObVlRU1NTU0HczsbOz27Bhw6pVq+rr63Upw1YP2j3CfBiMjanBMHuPERERFy5cePnyZVRU1IwZMygztDd5gQk2W1tbWFyowd27d/v27av3Ne6ooG90St9o24BSU1PT0NAgkUh4PJ5cLt+wYQO7fpcuXYKCgmJjY1UqVWlpKb32v4eHh4WFxZUrVyiJg4NDeHh4TExMeXk5QRBPnz7Vnq8KCwtraGg4evSoLmUnJ6dBgwbFxsbW1taWlpZSu1K5ublxudzU1FT4I222x9zc3KSkpNraWnAR+jbXsHFkRkYG3TAOh/Puu+9evnxZ+yL88ssvYWFhel/jjgr6Rqf0jdcPKAEBASRJsu9B5+7uHhcXN3369Lfeeis8PPz9999vttlDhw4VFBTY2dn169dv1KhRlFwsFu/cuXPKlCkikYha3vPDDz/w+fwePXqIRKLQ0NDs7GyN1rhc7qxZs7Zs2cKifPjw4by8POiRmlqXSCTr1q2bOHHiiBEjqE1PWBqpr69fv369vb29SCQ6deoUtRk1QRAuLi6LFy8ODQ11dnaGTdiA+fPn//vf/9a4FfLixYv09HT6n1hHBH3jzfUNPWdc2Pfl6TQwTpK1HVOmTPnuu+/oko8//njr1q3t0zsjht2XpzOBvsEOTMq2+R63CAs//vijhoRlo3zkjaKD+kanXSmLIEj7gxnK/wEG/8a2AjFF0Df0ATMUBEEMRgsylLS0NLglhnRiNJ550ZOsrKxm18IjnZuXL18SBPH/AgAA//9lL13V5zyD7gAAAABJRU5ErkJggg==)

The request (a coin) is passed to the first handler in the chain (e.g. five
pence handler), which will either process it or pass it on to its successor.
This continues until the request is processed or the end of the chain is

124 APPENDIX A. DESIGN PATTERN CATALOGUE
reached. The handler responsible for the final processing of the request
need not be known beforehand.
Example We implement the handling of different sized coins in this ex-
ample.
A Coin has a weight and a diameter. The handler will measure the
coins to decide if they should capture them.
type Coin struct {
Weight float
Diameter float
}
The CoinHandler interface defines the methods HandleCoin and
GetNext. Each coin handler has to implement the HandleCoinmethod.
The GetNextmethod will be common to all coin handlers. The Default-
CoinHandler type will implement this method as shown in the next
listing.
type CoinHandler interface {
HandleCoin(coin *Coin)
GetNext() CoinHandler
}
DefaultCoinHandler encapsulates common code for coin handlers
andmaintains a reference to the successor of the current coin handler. The

method GetNext returns the reference to the next coin handler. Default-
CoinHandler should be embedded by coin handlers to inherit themethod
GetNext and the slot for their successor. Coin handlers embedding De-
faultCoinHandler still have to implement Handle, making Handle
effectively an abstractmethod.
type DefaultCoinHandler struct {
Successor CoinHandler
}
func (this *DefaultCoinHandler) GetNext() CoinHandler {
return this.Successor
}

A.3. BEHAVIORAL PATTERNS 125
The FivePenceHandler is a concrete coin handler and embeds De-
faultCoinHanlder, inheriting GetNext and the slot Successor for a
successor coin handler. Since DefaultCoinHandler is a reference to a
*
DefaultCoinHandler object, the object needs to be instantiated,which
is done in NewFivePenceHandler.
type FivePenceHandler struct {
*DefaultCoinHandler
}
func NewFivePenceHandler() *FivePenceHandler {
return &FivePenceHandler{&DefaultCoinHandler{}}
}
The HandleCoinmethod checks if themeasurements of the coinmatch
the description of a five pence coin. If the match, a message is printed.
If the measurements don’t match the coin is forwarded to the next coin
handler (if there is one).
func (this *FivePenceHandler) HandleCoin(coin *Coin) {
if coin.Weight == 3.25 && coin.Diameter == 18 {
fmt.Println("Captured 5p")
} else if this.Successor != nil {

this.GetNext().HandleCoin(coin)
}
}
For brevitywe don’t showthe code of the handler handling ten pence,
twenty pence and so on. They are similar to the FivePenceHandler and
differ only in themeasurements of the coins and the statusmessage they
print.
The next listing shows how clients can create chains of coin handlers.
The handlers are instantiated and then the successor of each handler are
set. The bottom part of the listing shows how a coins is passed to the
beginning of the chain. Note that the coin counterfeit is not handled
by any handler. It virtually fell off the end of the chain.

126 APPENDIX A. DESIGN PATTERN CATALOGUE
h5 := NewFivePenceHandler()
h10 := NewTenPenceHandler()
h20 := NewTwentyPenceHandler()
h5.Successor = h10
h10.Successor = h20
twentyPence := &Coin{6.5, 24.5}
counterfeit := &Coin{10, 10}
h5.HandleCoin(twentyPence) //prints: Captured 10p
h5.HandleCoin(counterfeit) //prints:
Discussion Each HandleCoinmethod has to check if it has a successor
and will then forward the request. The else if branch of the Handle-
Coinmethods is repeated for each coin handler. This is redundant code,
which is bad style and it is easy to forget to forward a call to the next han-
dler. One solutionwould be to employ the TemplateMethod pattern (see
Appendix A.3.10). Each HandleCoinmethodwould return if it had han-
dled. Forwarding of the request would be done externally to the handlers
as shown in the next listing.
func Execute(handler CoinHandler, coin *Coin) {

handled := handler.HandleCoin(coin)
if handler.GetNext() != nil && !handled {
Execute(handler.GetNext(), coin)
}
}
Execute is a function controlling the chain’s execution. As long as the
request has not been handled Execute gets called recursively with the
current CoinHandler’s successor. Once the request (the coin) has been
handled or the current handler has no successor, the recursion stops.

A.3. BEHAVIORAL PATTERNS 127

### A.3.2 Command

Intent Represent and encapsulate all the information needed to call a
method at a later time, and decouple the sender and the receiver of a
method call.
Context Consider a remote control controlling lights and other equipment.
The remote has to be able toworkwith amultitude of electronic devices.
The remote has to be able to switch yet unknown devices on and off.
Adding newdevices should not result changes to the remote.
The solution is to turn requests (method calls) into Command objects.

![](data:image/png;base64,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)

The Client instantiates the Command object and provides the Receiver
(light) of the Command. The Invoker (remote control) is initialized with
a Concrete Command (on, off). The Receiver knows how to perform the
operations associatedwith carrying out a request (switch light on or off).
Inmany programming languages theCommand pattern is implemented
by encapsulating Commands in their own type. GO supports first class
functions and closures, which gives us a design alternative. We will be
showing both implementations in the following examples.
Example - Command as types In this first example we are encapsulating
the command for switching on the light in its own type.

128 APPENDIX A. DESIGN PATTERN CATALOGUE
We define Receiver as an interface. In our example the remote control
will be able toworkwith objects that can be switched on and off.
type Receiver interface {
SetOn(bool)
IsOn() bool
}
A Light has a field isOn. Light objects are the Receivers of requests.
SetOn and IsOn are accessors for the isOn field.
type Light struct {
isOn bool
}
func (this *Light) SetOn(isOn bool) {
this.isOn = isOn
}
func (this *Light) IsOn() bool {
return this.isOn
}
The interface Command defines themethod Execute that all concrete
commands have to implement. Execute performs operations on the com-
mand’s receiver.
type Command interface {
Execute()
}
OnCommand is a Concrete Command. OnCommandmaintains a reference
to a Receiver object. Themethod Execute sets the isOn field of receiver
to true.
type OnCommand struct {
receiver Receiver
}

func NewOnCommand(receiver Receiver) *OnCommand {
return &OnCommand{receiver}
}
func (this OnCommand) Execute() {
this.receiver.SetOn(true)
}

A.3. BEHAVIORAL PATTERNS 129
Objects of type RemoteControl are the Invokers of commands. Re-
moteControlmaintains a reference to a Command object. PressButton
call the Executemethod of that object. The behaviour of PressButton
depends on the dynamic type of command. This decouples the Invoker
fromthe Receiver of a request. A remote control is not limited to working
with lights.
type RemoteControl struct {
command Command
}
func (this *RemoteControl) PressButton() {
this.command.Execute()
}
The following listing ties it all together. remote and light get instan-
tiated; lightOnCmd is initialized with the object light; The remote’s
field command is set to the object lightOnCmd. PressButton calls the
Executemethod of the object lightOnCmd,which in turn sets the isOn
field of the light object to true.
remote := new(RemoteControl)
light := new(Light)
lightOnCmd := NewOnCommand(light)

remote.command = lightOnCmd
remote.PressButton()
if !light.isOn {
panic("light should be on")
}
Example - Command functions Adding the off-command requires the
addition of a newcommand type. In the followingwe showand alternative
using GO’s function pointers and closures. The Receiver interface and
the type Light are unchanged.
We define CommandFunc of function type. Functions that accept a
Receiver object can be usedwhere a CommandFunc is required.
type CommandFunc func(Receiver)

130 APPENDIX A. DESIGN PATTERN CATALOGUE
We redefine the Command type. Commands stillmaintain a reference to
the receiver of a request. The secondmember of Command command is of
function type CommandFunc.
type Command struct {
receiver Receiver
command CommandFunc
}
func NewCommand(receiver Receiver, command CommandFunc) *Command {
return &Command{receiver, command}
}
Command implements the method Execute. Execute calls the func-
tion commandwith receiver as parameter.
func (this *Command) Execute() {
this.command(this.receiver)
}
Newcommands can implemented as functions:
func OffCommand(receiver Receiver){
receiver.SetOn(false)
}
//somwhere else:
lightOffCmd := NewCommand(light, OffCommand)
orwith Closures:
lightOnCmd := NewCommand(light, func(receiver Receiver){
receiver.SetOn(true)
})
Discussion The first example follows the description as it is given in

Design Patterns. It is apparent that adding a new command type can be
quite elaborate. The second example shows, that it is easier to add new
commands with function pointers and Closures than having to define a
newcommand type.
Having a typewith the commandmethod has some advantages over
having functions only. Command objects can store state. Functions in GO
can store state only in static variables defined outside the function’s scope.

A.3. BEHAVIORAL PATTERNS 131
Variables defined inside the function’s scope cannot be static; they are ini-
tialized every time the function is called. The problemwith static variables
is that they can be altered externally too. The function cannot rely on the
variable’s state. Having a Command type allows to use helpermethods and
inheritance for reuse. This possibility is not given to functions. Subtypes
could override the helper functions (Template Method) or override the
algorithmbut reuse the helper functionality (Strategy).

132 APPENDIX A. DESIGN PATTERN CATALOGUE

### A.3.3 Interpreter

Intent Interpreter specifies howto evaluate a sentence in a language. The
pattern allows the grammar to be represented in an easily extended fashion.
Context Consider an application for evaluating complex boolean expres-
sions. Such expressions can be AND, OR or NOT and others.
The Interpreter pattern defines a language for expressions and shows
how to evaluate structures of expressions. A language consists of a vocabu-
lary (the words or expressions) and a grammar (the rules how expressions
can be combined). The following diagramshows the structure of the Inter-
preter pattern:

![](data:image/png;base64,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)

Expression defines the common interface for the vocabulary. There
are Terminal Expressions (constants, variables) and Non-terminal Expressions
(And, Or,Not). Non-terminal Expressions contain other Expressions. Ex-
pression can be combined according to the Composite pattern (see Ap-
pendix A.2.3) to create nested expressions or expression trees, with Ter-
minal Expressions being leafs and Non-terminal Expressions being nodes.
The Interpretmethod ofNon-terminal Expressions calls recursively its chil-
dren’s Interpretmethods. The recursion stops at Terminal Expressions. The
result of the interpretation of an Expression depends on the Context passed
to Interpret of the Expression tree.

A.3. BEHAVIORAL PATTERNS 133
Example In this example we implement a simple boolean expression
interpreterwith AND-, OR-NOT expressions (Non-Terminal Expressions),
and variables and constants (Terminal Expressions).
A Context stores two Variables. Each having a boolean value. The
result of the evaluation of the expression tree depends on the value of a
context object’s variables.
type Context struct {
x, y *Variable
xValue, yValue bool
}
The Assign methods are convenience methods for initialization of a
context object.
func (this *Context) AssignX(variable *Variable, xValue bool) {
this.x = variable
this.xValue = xValue
}
func (this *Context) AssignY(variable *Variable, yValue bool) {
this.y = variable
this.yValue = yValue
}
GetValue accepts a variable and returns the value of the according
variable.
func (this *Context) GetValue(variable *Variable) bool {
if variable == this.x {

return this.xValue
}
if variable == this.y {
return this.yValue
}
return false;
}
The following listings showthe source code of expressions.
BooleanExpressions are used to build the expression tree. Expres-
sions are composite. An expression can consist of other expressions. Every
expression has to implement themethod Evaluate returning a boolean.
The parameter of type Context contains the names and values of the
*

134 APPENDIX A. DESIGN PATTERN CATALOGUE
variables the expression is to be evaluatedwith.
type BooleanExpression interface {
Evaluate(*Context) bool
}
A NotExpression contains a field operand of type BooleanExpres-
sion. NewNotExpression is the constructor method for NotExpres-
sions setting the operand. The method Evaluate returns the negated
value of its operand’s Evaluatemethod. Evaluate is called recursively
passing along the context parameter. The expression tree is traversed
and the evaluation starts at the leaves.
type NotExpression struct {
operand BooleanExpression
}
func NewNotExpression(operand BooleanExpression) *NotExpression {
return &NotExpression{operand}
}
func (this *NotExpression) Evaluate(context *Context) bool {
return !this.operand.Evaluate(context)
}
The constructormethods of the other expressions are similar. We omit
themfor brevity.

An AndExpression has two operands of type BooleanExpression.
Evaluate calls its operands Evaluate method and concatenates their
values with the AND-operator &&. The type OrExpression is similar,
differing only in its Evaluate operator OR ||. We omit the code for
OrExpression.
type AndExpression struct {
operand1 BooleanExpression
operand2 BooleanExpression
}
func (this *AndExpression) Evaluate(context *Context) bool {
return this.operand1.Evaluate(context) && this.operand2.Evaluate(context)
}

A.3. BEHAVIORAL PATTERNS 135
The type Variable is a BooleanExpression with a name. The pa-
rameter context of Evaluate stores two variables and their according
values. Evaluate returns the value for the current Variable object this.
type Variable struct {
name string
}
func (this *Variable) Evaluate(context *Context) bool {
return context.GetValue(this);
}
A ConstantExpression stores a boolean value. Evaluate returns
that value.
type Constant struct {
operand bool
}
func (this *Constant) Evaluate(context *Context) bool {
return this.operand
}
The types Constant and Variable are leafs in the expression tree,
And-, Or-, and NotExpression are composite expressions, containing
other expressions.
The following listing showthe construction of an expression tree and
its evaluation.
1 func main() {
2 var expression BooleanExpression
3 x := NewVariable("X")
4 y := NewVariable("Y")

5 expression = NewOrExpression(
6 NewAndExpression(NewConstant(true), x),
7 NewAndExpression(y, NewNotExpression(x)))
8
9 context := new(Context)
10 context.AssignX(x, true)
11 context.AssignY(y, false)
12
13 result := expression.Evaluate(context)
14 fmt.Println("result:", result)
15 }

136 APPENDIX A. DESIGN PATTERN CATALOGUE
The expression object assembled in lines 5-7 represents:
(true AND x) OR (y AND (NOT x)). The context object is initial-
ized with the Variable objects x and y. x’s value is true and y’s value
is false. Evaluating the expression object with the context object
results in true ((true AND true) OR (false AND (NOT true)).
Discussion This implementation of the Interpreter pattern is like inmost
other object-oriented languages. Clients build an abstract syntax tree (a
sentence in the grammar to be interpreted). A Context object is initialized
and passed alongside the sentence to the Interpret operation. The Interpret
operation stores and accesses the Interpreters state in the Context object at
each node.
As a design alternative, the Interpret operation (in this example Evalu-
ate) could be implemented as a function. No separate type necessary to

hold that operation as amethod.

A.3. BEHAVIORAL PATTERNS 137

### A.3.4 Iterator

Intent Access the elements of an aggregate object sequentially without
exposing its underlying representation.
Context Aggregate object should give away to access its elementswith-
out exposing its internal structure.
The Iterator pattern is a solution for that problem. There aremultiple
ways to implement Iterator. We present GO’s built-in iterators for certain
types, we show how to implement an internal iterator and we demonstrate
howexternal iterators can be implemented.
Example - Built-in iterators GO provides built-in iteration functionality
for maps, arrays/slices, strings and channels. for loops with a range
clause iterate through all entries of anmap, array/slice or string, or values
received on a channel. We showan example for each data structure.
Maps days is amap with keys being the days of the weeks as strings

and values being the the index of the days of the week as integers. The for
look iterates over the daysmap. On each loop key and value are assigned
the values range returns. range formaps returns two values: themap’s key
and the according value.
days := map[string]int{ "mon": 0, "tue": 1, "wed": 2, "thu": 3,
"fri": 4, "sat": 5, "sun": 6}
for key, value := range days {
fmt.Printf("%v : %d,", key, value)
}
//Prints: fri : 4,sun : 6,sat : 5,mon : 0,wed : 2,tue : 1,thu : 3,

138 APPENDIX A. DESIGN PATTERN CATALOGUE
Arrays/Slices words is an array of strings. The for loop iterates
over the elements. range returns the index of the current element and
the element itself.
words := []string{"hello", "world"}
for index, character := range words {
fmt.Printf("%d : %v,", index, character)
}
//Prints: 0 : hello,1 : world,
Strings helloWorld is a string with 11 characters. The for loop
iterates over each character and prints it on the console. The range clause
returns the index of the current character and the character itself.
helloWorld := "hello world"
for index, char := range helloWorld {
fmt.Printf("%d : %c,", index, char)
}
//Prints: 0 : h,1 : e,2 : l,3 : l,4 : o,5 : ,6 : w,7 : o,8 : r,9 : l,10 : d
Channels strings is a channel of strings with a buffer size of 2.
Two strings are sent on the channel and the channel is closed for input.

The for loop prints the received values on the channel strings. range on a
channel returns the values received on the channel in first-in first-out order.
strings := make(chan string, 2)
strings <- "hello"
strings <- "world"
close(strings)
for aString := range strings {
fmt.Printf("%v, ", aString)
}
//Prints: hello, world,

A.3. BEHAVIORAL PATTERNS 139
Example - Internal Iterator The task of an internal, or passive, iterator
is to execute a function on each element of a collection. Clients pass an
operation to the internal iterator and the iterator performs that operation
on every element of the collection. In this example we demonstrate the
internal iterator of the type Vector.
list is of type Vector and two string are added to it. AClosure printing
the current element is passed to list’s Domethod.
list := new(vector.Vector)
list.Push("hello")
list.Push("world")
list.Do(func(e interface{}){
fmt.Println(e)
})
2
The code of Vector’smethod Do is shown below:
func (p *Vector) Do(f func(elem interface{})) {
for _, e := range *p {
f(e)
}
}
Do accepts a function with one parameter of the empty interface type
interface. Do iterates over the aggregate and calls the passed in function

with the current element of the aggregate. range can be used here because
type Vector is an extension of array type (type Vector []interface).
2
Taken fromVector’s package documentation [6]

140 APPENDIX A. DESIGN PATTERN CATALOGUE
Example - External Iterator Here we show an implementation of an ex-
ternal iterator for slices. This is a demonstrative example, as seen above
slices can be traversedwith the for-range clause.
The Iterator interface defines Next and HasNext controlling the
iterators position in the aggregate.
type Iterator interface {
Next() interface{}
HasNext() bool
}
SliceIterator maintains a reference to the slice slice. index
stores the current location of the iterator. On instantiation of SliceItera-
tor the slice slice has to be set and indexwill be initialized to 0.
type SliceIterator struct{
slice []interface{}
index int
}
func NewSliceIterator(slice []interface{}) Iterator {
this := new(SliceIterator)
this.slice = slice
return this
}
The method HasNext compares the current position with the length

of the slice. HasNext returns false if there are nomore elements in the
slice, true otherwise.
func (this *SliceIterator) HasNext() bool {
return this.index < len(this.slice)
}
Next returns the current element of the slice and increments index. If
the current index points to an invalid element in the slice it panics.
func (this *SliceIterator) Next() interface{} {
if this.index >= len(this.slice) {
panic("NoSuchElement")
}
element := this.slice[this.index]
this.index++
return element
}

A.3. BEHAVIORAL PATTERNS 141
The next listing shows how a SliceIterator can be used. A slice
aSlice containing two strings is created (behind the scenes: an array of
length two gets created, the values assigned and then the array gets sliced).
ASliceIterator object iter gets instantiatedwith the just created slice.
iter handles the internal iteration of the slice and knows its the current
position in the slice. The for-loop runs as long as there is another element in
the slice. Inside the loop the iterator advances and prints the next element
in the slice.
aSlice := []interface{}{"hello", "world"}
iter := NewSliceIterator(aSlice)
for iter.HasNext() {
current := iter.Next()
fmt.Println(current)
}

142 APPENDIX A. DESIGN PATTERN CATALOGUE
3
Example - Iterationwith channels In an earlier version , the type Vector
provided an external Iterator by sending its elements on a channel. The
channel containing the elements can then be used to iterate of the vector’s
elements.
Themethod Iter creates and returns a channel containing the vectors
elements. The sending of elements to the channel happens in the private
method iterate, which runs in a separate goroutine. The returned chan-
nel c is an output channel (the operator <- is on the left).
func (p *Vector) Iter() <-chan interface{} {
c := make(chan interface{})
go p.iterate(c)
return c
}
The method iteratate iterates over the elements of the vector and
sends themon the channel c. c is defined as an input channel (the operator
<- is on the right).
func (p *Vector) iterate(c chan<- interface{}) {
for _, v := range *p {
c <- v
}
close(c)

}
Clients call Iter on vector objects and receive a channel containing the
vector’s elements. This channel can be used to iterate over the vector’s
elements:
for element := range aVector.Iter() {
DoSomething(element)
}
Discussion GO provides amechanismto iterate over some of the built-in
types. External iterators are still necessary and their implementation is
similar to Design Patterns.
3
Themethod Iter has been removed fromVectorwith the release fromthe 14/07/2010.
[3]

A.3. BEHAVIORAL PATTERNS 143

### A.3.5 Mediator

Intent Provide a central hub to guide the interactions between many
objects. Reducemany-to-many to one-to-many relationships.
Context Consider implementing the communication between themem-
bers of parliament. This communication would create dependencies if
everymembermaintained a reference to the othermembers.
TheMediator pattern is a solution for this problem, encapsulating the
collective behaviour in a separatemediator object. The following diagram
depicts the structure of themediator pattern.

![](data:image/png;base64,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)

TheMediator interface (forum) defines an interface for communication
with Colleague objects. ConcreteMediators (political forum) implement the
behaviour defined in theMediator interface. ConcreteMediatorsmaintain
references to their colleagues. Each Concrete Colleague (member of parlia-
ment, prime minister) knows its Mediator object. Concrete Colleagues
communicate through theirMediatorwith their Colleagues.
Example In this examplewe showhowto implement a political forum,
wheremembers of parliament can sendmessages to a single recipient or to
everymember of the forum. Themembers of the forumdon’t have to know
all othermembers. A forumdistributesmessages between participants—
we call themcolleagues to gowith the pattern.

144 APPENDIX A. DESIGN PATTERN CATALOGUE
Forum lists a small set ofmethods concrete forums have to implement.
Forum is theMediator interface. Send is for broadcasts, accepting the name
of the sender and themessage to be send to all colleagues. SendFromTo
is for directed message sends, from one colleague to the other. The Add
message is used for colleagues to join the forum.
type Forum interface {
Send(from, message string)
SendFromTo(from, to, message string)
Add(c Colleague)
}
Forum does not define how concrete forums have to maintain their
members. PoliticalForum keeps a map, with the name of the colleague
as keys, and Colleague objects as values.
type PoliticalForum struct {
colleagues map[string]Colleague
}
NewPoliticalForum instantiates a PoliticalForum and initializes
themap of colleagues. The built-in functionmake reservesmemory for the
map and returns a pointer to it.

func NewPoliticalForum() *PoliticalForum {
return &PoliticalForum{make(map[string]Colleague)}
}
The Add method adds the passed in Colleague object to the map
and sets the colleague’s forum. Forum and Colleague have a two-way
relationship. Forums knowtheirmembers and colleagues knowinwhich
forumthey are.
func (this *PoliticalForum) Add(colleague Colleague) {
this.colleagues[colleague.GetName()] = colleague
colleague.SetForum(this);
}
Send does a broadcast, forwarding themessage to all colleagues of the
political forum, except to the sender.

A.3. BEHAVIORAL PATTERNS 145
func (this *PoliticalForum) Send(sender, message string) {
for name, colleague := range this.colleagues {
if name != sender {
colleague.Receive(sender, message)
}
}
}
SendFromTo does directed message sends. The method gets the re-
ceiver fromthemap of colleagues, checks if the receiver exists and sends
the message to the colleague. Send and SendFromTo are the mediating
methods. They decouple the colleagues fromeach other. Colleagues don
not have to keep track of other colleagues.
func (this *PoliticalForum) SendFromTo(sender, receiver, message string) {
colleague := this.colleagues[receiver]
if colleague != nil {
colleague.Receive(sender, message)
} else {
fmt.Println(sender, "is not a member of this political forum!")
}
}
Colleague defines the interface members of forums have to imple-
ment. Colleagues have a name for identification, can be member of a

forum, can sendmessages to a single recipient or to allmembers of their
forum, and they can receivemessages fromothermembers of a forum.
type Colleague interface {
GetName() string
SetForum(forum Forum)
Send(to, message string)
SendAll(message string)
Receive(from, message string)
}
MemberOfParliament (MOP) are Concrete Colleagues. MOPs have a
name and a forum.
type MemberOfParliament struct {
name string
forum Forum
}

146 APPENDIX A. DESIGN PATTERN CATALOGUE
NewMemberOfParliament instantiates a newMOP and sets its name.
GetName and SetForum are public accessors to the MOP’s name and
forum.
func NewMemberOfParliament(name string) *MemberOfParliament {
return &MemberOfParliament{name:name}
}
func (this *MemberOfParliament) GetName() string {
return this.name
}
func (this *MemberOfParliament) SetForum(forum Forum) {
this.forum = forum
}
Send and SendAll forward themessages to their forum,which then
determines the the right recipients.
func (this *MemberOfParliament) Send(receiver, message string) {
this.forum.SendFromTo(this.name, receiver, message);
}
func (this *MemberOfParliament) SendAll(message string) {
this.forum.Send(this.name, message);
}
Receive prints themessage on the console including the sender and
the receiver.
func (this *MemberOfParliament) Receive(sender, message string) {

fmt.Printf("The Hon. MP %v received a message from %v: %v\n",
this.name, sender, message)
}
PrimeMinister (PM) is a specialization of MOP. By embedding MOP,
PM inherits all of MOP’s functionality.
type PrimeMinister struct {
*MemberOfParliament
}
func NewPrimeMinister(name string) *PrimeMinister {
return &PrimeMinister{NewMemberOfParliament(name)}
}
PrimeMinister defines its own Receivemethod, overridingMOPs,
to print amore suitablemessage.
func (pm *PrimeMinister) Receive(sender, message string) {
fmt.Printf("Prime Minister %v received a message from %v: %v\n",

A.3. BEHAVIORAL PATTERNS 147
pm.name, sender, message)
}
In the following listing, a forumand three colleagues get instantiated.
The colleagues are added to the forum, by which the forum sets itself as
the colleagues forum.
forum := NewPoliticalForum()
pm := NewPrimeMinister("Prime")
mp1 := NewMemberOfParliament("MP1")
mp2 := NewMemberOfParliament("MP2")
forum.Add(pm)
forum.Add(mp1)
forum.Add(mp2)
Colleagues sendingmessages are send to their forums,which distribute
them to the receivers. The commented lines show the console output of
each call.
pm.SendAll("Hello everyone")

| //The Hon. MP | M P 2 | received a message from | P r i m e : Hello everyone |
| --- | --- | --- | --- |
|  | M P 1 |  |  |

mp1.Send("MP2", "Hello back")
//The Hon. MP M P 2 received a message from M P 1 : Hello back
mp2.Send("Prime", "Dito")
//Prime Minister P r i m e received a message from M P 2 : Dito
pm.Send("MP1", "Bullsh!t")
//The Hon. MP M P 1 received a message from P r i m e : Bullsh!t
mp1.SendAll("I second")
//Prime Minister P r i m e received a message from M P 1 : I second
//The Hon. MP M P 2 received a message from M P 1 : I second
Discussion In implementing theMediator pattern, the interface Forum
is optional. The political forumwould still be themediator. Other forum
types could be implemented against that interface in connectionwith the
colleague interface. The concrete colleagueswe implemented here could
then be used to interact through any type of forum.
Colleagues are only able to be member of one forum at a time. This

restriction could be lifted by colleagues being able tomaintain a collection
of forums.

148 APPENDIX A. DESIGN PATTERN CATALOGUE

### A.3.6 Memento

Intent Brings an object back to a previous state by externalizing the ob-
ject’s internal state.
Context Consider you decide to take a phone apart in order to replace an
internal part. The problemis to get it back to its original state. A solution is
taking pictures while dismantling the phone. Each picture is amemento, a
reminder or reference of howan object should look.

![](data:image/png;base64,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)

The above shows the structure of theMemento pattern. The Originator
(the phone) has internal state (the phone’s parts). CreateMemento instan-
tiates a Memento (a picture) and the Memento stores the current state of
the Originator. The Caretaker (the client taking a phone apart) asks the
Originator for a Memento. The Caretaker uses the Mementos to restore
the Originator. Originator andMemento should be in the same package,
separate fromthe Caretaker, to prohibit Caretakers changing the state of
Mementos.
Example In this examplewe remove parts froma phone and showhow
theMemento pattern is used to restore the phone to a certain state in the
dismantling process.

A.3. BEHAVIORAL PATTERNS 149
4
Phone objects are the Originators. Phone stores a map of Parts . The
map of parts represents the internal state.
type Part string
type Phone struct {
parts map[int]Part
}
RemovePart removes parts fromthe phone’s partsmap. To remove
entries from maps multiple assignment is used in GO. If the additional
boolean on the right is false, the entry is deleted. No error occurs if the key
does not exist in themap.
func (this *Phone) RemovePart(part int) {
this.parts[part] = "", false
}
TakePicture creates a Picture object with the phone’s current in-
ternal state. Picture is the Memento, storing the phone’s current state.
Clients call this method to create a snapshot of the phone in its current
state. RestoreFromPicture replaces the phone’s parts with those of the
picturewhen itwas taken.
func (this *Phone) TakePicture() *Picture {

return NewPicture(this.parts)
}
func (this *Phone) RestoreFromPicture(picture *Picture) {
this.parts = picture.getSavedParts()
}
Pictures areMementos for phones. Picture maintains, like Phone, a
map of Part objects to store the phone’s state.
type Picture struct {
parts map[int]Part
}
NewPicture instantiates a picture and copies the phone’s parts. The
creation of a newmap and copying of the elements is necessary. Consider
assigning partsToSafe to this.parts. Both variableswould point to
the samemap object. Changes to phones parts would result in a change in
picture’s parts,which is not intended.
4
For simplicity Part is an extension of string

150 APPENDIX A. DESIGN PATTERN CATALOGUE
func NewPicture(partsToSave map[int]Part) *Picture {
this := new(Picture)
this.parts = make(map[int]Part)
for index, part := range partsToSave {
this.parts[index] = part
}
return this
}
The method getSavedState is package local, e.g. not visible out-
side the current package (identifier startswith lower case character). The
method returns the parts of the phonewhen the Picture object thiswas
created. Caretakers residing outside Picture’s package cannot access the
Pictures’s state.
func (this *Picture) getSavedParts() map[int]Part {
return this.parts
}
The next listings show how Pictures can be used to restore a Phone.
NewPhone creates a Phone object and adds four parts.
func NewPhone() *Phone {
this := new(Phone)
this.parts = make(map[int]Part)
this.parts[0] = Part("body")
this.parts[1] = Part("display")
this.parts[2] = Part("keyboard")

this.parts[3] = Part("mainboard")
return this
}
The following listing demonstrates the Caretaker. pictures is amap of
Picture objects storing theMementos. An initialmemento of the phone’s
original state is created, and two additional pictures are taken during
the disassembly of the phone object (RemovePart). TakePicture cre-
ates a Picture object storing the phone’s current parts. The commented
lines describe the state of the phone after each removal. The last three
lines show how the phone can be restored to a given state by the care-
taker. The caretaker selects the necessary state and passes it to the phone’s
RestoreFromPicturemethod.

A.3. BEHAVIORAL PATTERNS 151
phone := NewPhone()
pictures := make([]*Picture, 3)
pictures[0] = phone.TakePicture()
phone.RemovePart(0)
phone.RemovePart(2)
//mainboard and display remaining
pictures[1] = phone.TakePicture()
phone.RemovePart(1)
pictures[2] = phone.TakePicture()
//mainboard remaining
phone.RemovePart(3)
//phone dismantled
phone.RestoreFromPicture(pictures[2])
phone.RestoreFromPicture(pictures[1])
phone.RestoreFromPicture(pictures[0])
Discussion In the Memento pattern, Memento objects have two inter-
faces: a narrow interface for the Caretaker and a wide one for Originators.
In C++ Originator is a friend of Memento gaining access to the narrow
interface. Themethods in the wide interface are to be declared public. Java
has no functionality that could be used. In GO we define the wide inter-
facewith publicmethods and the narrow onewith package localmethods.

Memento and Originator should reside in same package and Caretakers
should be external, to protectMementos frombeing accessed.

152 APPENDIX A. DESIGN PATTERN CATALOGUE

### A.3.7 Observer

Intent A one-to-many relationship between interesting and interested
objects is defined. When an event occurs the interested objects receive a
message about the event.
Context Consider a football game. Players and referees know the last
position of the ball. They should not constantly poll the ball for its current
position. Whenever the ball moves all interested participants should be
notified.
Asolution is theObserver pattern. The image belowshows the structure
of the observer pattern.

![](data:image/png;base64,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)

Observers (players, referees) registerwith Subjects (balls). The Concrete
Subjects informs their Observers about changes. Concrete Observers get
the changes from the Subject that notified them. All Subjects have the
Attach, Detach and Notify operations in common. The functionality is

A.3. BEHAVIORAL PATTERNS 153
encapsulated in a Default Subject type. Every type that embeds Default
Subject is automatically a Concrete Subject.
Example We showhowa ball can observed by players. The players get
notified about the change of position and poll the ball for the new position.
The Subject interface lists themethods Attach, Detach and Notify.
type Subject interface {
Attach (observer Observer)
Detach (observer Observer)
Notify()
}
DefaultSubject implements themethods defined in the Subject in-
terface. DefaultSubjectmaintains a collection of observers. Default-
Subject is not meant to be instantiated directly, but to be embedded.
Types embedding DefaultSubject will gain the necessary functionality
to be observable. In this example the type FootBall (shown further down)
embeds DefaultSubject.
type DefaultSubject struct {
observers *vector.Vector
}

func NewDefaultSubject() *DefaultSubject {
return &DefaultSubject{observers:new(vector.Vector)}
}
Attach and Detach handle adding and removing of observers.
func (this *DefaultSubject) Attach(observer Observer) {
this.observers.Push(observer)
}
func (this *DefaultSubject) Detach(observer Observer) {
for i := 0; i < this.observers.Len(); i++ {
currentObserver := this.observers.At(i).(Observer)
if currentObserver == observer {
this.observers.Delete(i)
}
}
}

154 APPENDIX A. DESIGN PATTERN CATALOGUE
Notify informs all observers in the list. The method loops over all
observers and calls their Updatemethod,which triggers the observers to
poll their subject’s state.
func (this *DefaultSubject) Notify() {
for i := 0; i < this.observers.Len(); i++ {
observer := this.observers.At(i).(Observer)
observer.Update()
}
}
Position objects represent a point in the 3-dimensional spacewith x,
y and z coordinates.
type Position struct {type Position struct {
x, y, z int
}
func NewPosition(x, y, z int) *Position {
return &Position{x, y, z}
}
FootBall is a Concrete Subject. It inherits the necessary functional-
ity from embedding DefaultSubject. A FootBall knows its current
position.
type FootBall struct {
*DefaultSubject
position *Position
}
func NewFootBall() *FootBall {
return &FootBall{DefaultSubject:NewDefaultSubject()}
}

GetPosition provides public access the FootBall’s current position.
SetPosition updates the the FootBalls position and informs all its
observers about the change, which in turn query the football for its new
position.
func (this *FootBall) GetPosition() *Position {
return this.position
}

A.3. BEHAVIORAL PATTERNS 155
func (this *FootBall) SetPosition(position *Position) {
this.position = position
this.Notify()
}
The Observer interface defines one method: Update. The Update
method is called by subject’s Notifymethod.
type Observer interface {
Update()
}
Player is a Concrete Observer. Players have a name,maintain a refer-
ence to a FootBall and remember the the ball’s last position.
type Player struct {
name string
lastPosition *Position
ball *FootBall
}
func NewPlayer(name string, ball *FootBall) *Player {
this := new(Player)
this.name = name
this.ball = ball
return this
}
Update updates the player’s last known position of the ball and prints
amessage on the console.
func (this *Player) Update() {
this.lastPosition = this.ball.GetPosition()
fmt.Println(this.name, "noticed that ball has moved to", this.lastPosition)
}

156 APPENDIX A. DESIGN PATTERN CATALOGUE
In the following listing a ball and three players are created. The players
get names and a reference to the ball. The players are attached to the ball to
be notified about the ball’s position changes. The first time the ball changes
position all three players get notified and print amessage; the second time
only player1 and player3, since player2 is no longer an observer of the ball.
var aBall = NewFootBall()
var player1 = NewPlayer("player1", aBall)
var player2 = NewPlayer("player2", aBall)
var player3 = NewPlayer("player3", aBall)
aBall.Attach(player1)
aBall.Attach(player2)
aBall.Attach(player3)
aPosition := NewPosition(1, 2, 3)
aBall.SetPosition(aPosition)
aBall.Detach(player2)
aBall.SetPosition(NewPosition(2, 3, 4))
Discussion Java’s abstract classes combine the definition of interface and

default implementation. GO does not have abstract classes. Embedding can
be used, even though it is not as comfortable. One advantage of embedding
becomes apparent in this implementation of the Observer pattern. Tomake
a type observable, only DefaultSubject needs to be embedded. Java sup-
ports single class inheritance, but in GO multiple types can be embedded.
Types that are doing their responsibility can easily be made observable.
And the embedding type can still override the embedded type’smembers
to provide different behaviour.
TheObserver pattern iswidely used in Java libraries (Listeners in Swing
for example), but not in GO, as far aswe can see.

A.3. BEHAVIORAL PATTERNS 157

### A.3.8 State

Intent Change an object’s behaviour by switching dynamically to a set of
different operations.
Context Consider anMP3 player. Depending onwhat the player is doing
(being in standbymode or playingmusic) a click on the play button should
change the player’s behaviour.
The State pattern is a solution for this problemof dynamic reclassifica-
tion.

![](data:image/png;base64,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)

A Context (MP3 player) has a State. Asking the Context to execute its
functionality (clicking the play button),will execute the Context’s current
State’s behaviour. The behaviour depends on the dynamic type of the State
object. Concrete States (playing, standby) handle the State transitions (if
play is clicked while the player is in standby, the player wakes up and is
set into the play state).
GO offers two alternatives for implementing the State pattern. One way
is to encapsulate states in state objects of different type, the other is to use
functions. We show source code for both alternatives by implementing the
above outlinedMP3 player example.
Example - States as types In this example, the states, that anMP3 player
can be in, are implemented as separate types. TheMP3 playermaintains a
reference to a state object and executes the states behaviour.

158 APPENDIX A. DESIGN PATTERN CATALOGUE
All States have to implement the Handlemethod. The state objects
are Flyweight objects (see Appendix A.2.6), they can be shared between
contexts. State objects do not store their Context. The context the states
operate on is passed in on every call of Handle. The concrete states we
implement in this example could be implemented as Singleton objects (see
appx:singleton). We omit this in order to focus on the State pattern
itself.
type State interface {
Handle(player *MP3Player)
}
Standby is a Concrete State. Upon calling of Handle, a message is
printed (symbolizing the actions to wake up the MP3 player) and the
player’s state is changed to Playing. Note that a newPlaying object is
created each time Handle is called. The concrete states should be Singleton
as explained above.
type Standby struct{}
func (this *Standby) Handle(player *MP3Player) {

fmt.Println("waking up player")
player.SetState(new(Playing))
}
Playing is another Concrete State. Handle prints outmessages on the
console describingwhat the player does. After playingmusic, the player is
set into Standby.
type Playing struct{}
func (this *Playing) Handle(player *MP3Player) {
fmt.Println("start playing music")
fmt.Println("music over")
fmt.Println("waited ten minutes")
fmt.Println("send player to sleep")
player.SetState(new(Standby))
}
The MP3Player type is the Context. The playermaintains a reference to
a State object. NewMP3Player instantiates a newMP3Player object and
sets its initial state. SetState provideswrite access to the player’s state.

A.3. BEHAVIORAL PATTERNS 159
SetState is public to enable state transition for package external State
types (not shown).
type MP3Player struct {
state State
}
func NewMP3Player(state State) *MP3Player {
this := new(MP3Player)
this.state = state
return this
}
func (this *MP3Player) SetState(state State) {
this.state = state
}
PressPlay calls the current state’sHandlemethod. The behaviour of
PressPlay depends on the dynamic type player’s current state.
func (this *MP3Player) PressPlay() {
this.state.Handle(this)
}
AMP3Player is initializedwith state Playing. The first call of Press-
Play will perform the behaviour defined in Playing.Handle and will
set player into Standby. The second call of PressPlay will wake up
the player (Standby’s behaviour).
player := NewMP3Player(new(Playing))
player.PressPlay()
player.PressPlay())

Example - States as functions GO provides first class functions. Instead
of defining separate types for each state, states can defined in functions.
TheMP3 playermaintains a reference to a function instead of to an object.
Instead of being an interface with a singlemethod, States are of func-
tion type.
type State func(player *MP3Player)
Standby is a function conforming to the function type State. The
functionality performed by Standby is as before. Note that the parameter

160 APPENDIX A. DESIGN PATTERN CATALOGUE
Playing passed to SetState is a function.
func Standby(player *MP3Player) {
fmt.Println("waking up player")
player.SetState(Playing)
}
Here Playing is also a function of type State. As with Standby,
Playing has the same behaviour as before.
func Playing(player *MP3Player) {
fmt.Println("start playing music")
fmt.Println("music over")
fmt.Println("waited ten minutes")
fmt.Println("send player to sleep")
player.SetState(Standby)
}
MP3Player is syntactically like before. The difference lies in the type of
itsmember state, nowit is function type, itwas interface type. NewMP3-
Player and SetState are syntactically like before, but their parameters
state are nowof function type instead of interface type.
type MP3Player struct {
state State
}
PressPlay executes the states behaviour directly, by passing the

MP3Player object to this.state,which is a function.
func (this *MP3Player) PressPlay() {
this.state(this)
}
Below, a new MP3Player object is created and initialized with Play-
ing. PressPlay calls the function stored in the player’s state.
player := NewMP3Player(Playing)
player.PressPlay()
player.PressPlay()
Discussion Creating types for each state can become a burden, since
each state-type should be implemented as a singleton or flyweight. Using
functions to encapsulate state specific behaviour avoids this overhead.

A.3. BEHAVIORAL PATTERNS 161
In addition, the code is easier to follow. With state objects the context’s
behaviour is dependent on the dynamic type of the state objects. Figuring
out which Execute method is called at a certain point can be a daunting
task.

162 APPENDIX A. DESIGN PATTERN CATALOGUE

### A.3.9 Strategy

Intent Decouple an algorithm from its host by encapsulating the algo-
rithminto its own type.
Context Consider a simple calculator that accepts two numbers as input
and a button which defines the calculation to done with the two numbers.
Newtypes of calculation need to be addedwithout having to change the
calculator.
The solution is to encapsulate the calculations in independent constructs
and provide a uniforminterface to them.

![](data:image/png;base64,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)

Strategy provides an interface for supported algorithms. Concrete Strate-
gies (add, subtract,multiply) encapsulate algorithms. The Context’s (calcu-
lator) behaviour depends on the dynamic type of the Strategy and can be
changed dynamically. In GO, Strategies can be encapsulated in objects and
in functions. We demonstrate both alternativeswith the above described
calculator example.
Example - Strategies as objects In this example we implement the strate-
gies (add,multiply and subtract) as their own types.
Strategy defines the common interface for all algorithms. Themethod
Execute accepts two integers and returns the result of the operation as an
integer.
type Strategy interface {
Execute(int, int) int
}

A.3. BEHAVIORAL PATTERNS 163
Add, Subtract, and Multiply are Concrete Strategies. Each having an
Executemethod that conforms to the Strategy interface. The Execute
methods encapsulate the algorithms.
type Add struct{}
func (this *Add) Execute(a, b int) int {
return a + b
}
type Subtract struct{}
func (this *Subtract) Execute(a, b int) int {
return a - b
}
type Multiply struct{}
func (this *Multiply) Execute(a, b int) int {
return a * b
}
Calculator is the Context. A Calculator object maintains a refer-
ence to a Strategy object and provides publicwrite access. This enables
package external Clients to configure Calculator’s strategy.
type Calculator struct {
strategy Strategy
}
func (this *Calculator) SetStrategy(strategy Strategy) {
this.strategy = strategy
}
Clients call Calculate on a Calculator object, which in turn exe-

cutes the current strategy’s Execute method. Calculate’s behaviour
depends on the dynamic type of the object strategy.
func (this *Calculator) Calculate(a, b int) int {
return this.strategy.Execute(a, b)
}
The following listing demonstrates how Clients alter the Calculators
behaviour by configuring its strategy. The calculator object is the con-
text that a strategy resides in. Clients set the calculator’s strategy. Upon
calling Calculate on the context, the context calls its current strategy’s
Executemethod and passes two numbers. The results differ since the dy-
namic type of calculator’s strategy is differentwith each call of Calculate.

164 APPENDIX A. DESIGN PATTERN CATALOGUE
calculator := new(Calculator)
calculator.SetStrategy(new(Add))
calculator.Calculate(3, 4) //result: 7
calculator.SetStrategy(new(Subtract))
calculator.Calculate(3, 4) //result: -1
calculator.SetStrategy(new(Multiply))
calculator.Calculate(3, 4) //result: 12
Note that newinstances of the strategies are created. These strategies
wouldmake good Singletons (see Appendix A.1.5), but we kept the exam-
ple simple to not distract fromthe pattern at hand.
Example - Strategies as functions GO has first class functions. Strate-
gies can be encapsulated in functions and the Contexts store references to
strategy functions, instead of strategy objects.
Strategy is a function type. Every function accepting two and return-
ing one integer can be usedwhere a Strategy is required.
type Strategy func(int, int) int

Here Multiply is not a struct, but a function encapsulating the algo-
rithm. No additional types necessary.
func Multiply(a, b int) int {
return a * b
}
Add and Subtract are not shown here for brevity. They followthe same
pattern.
The definition of Calculator is the same as above. There is no syntac-
tical difference in SetState either. However, there is a semantic difference,
Calculator keeps a reference to a function instead of an object. Type
Strategy is of function type, not of interface type.
type Calculator struct {
strategy Strategy
}

A.3. BEHAVIORAL PATTERNS 165
Calculate differs in that it calls the strategy function directly instead
of referring to the strategy’smethod Execute.
func (this *Calculator) Calculate(a, b int) int {
return this.strategy(a, b)
}
Clients pass functions to the context instead of Strategy objects. On
calling Calculate the context object calls its current strategy function and
forwards the two integers and returns the result.
calculator := new(Calculator)
calculator.SetStrategy(Add)
calculator.Calculate(3, 4) //result: 7
calculator.SetStrategy(Subtract)
calculator.Calculate(3, 4) //result: -1
calculator.SetStrategy(Multiply)
calculator.Calculate(3, 4) //result: 12
Discussion Defining a types and a method for each strategy is elabo-
rate compared to defining a function. Strategies could be Singletons, if
they don’t need to store context specific state. Encapsulating strategies

as functions removes the necessity for Singleton. If the strategies need to
store state, they are better implemented with types. Strategy functions
could store state in package-local static variables, but this would break
encapsulation, since other functions in the package have access to those
variables.
Encapsulation could be achieved by organizing each function in a sepa-
rate package, but this comes at a cost: each package has to be in a separate
folder, additionally the packages need to be imported before the strategy
functions can be used. This seems to be more work than implementing
separate types for the strategies.

166 APPENDIX A. DESIGN PATTERN CATALOGUE

### A.3.10 TemplateMethod

Intent The TemplateMethod design pattern can be usedwhen the outline
of an algorithmshould be static between classes, but individual stepsmay
vary.
Context Consider a framework for turn-based games. All turn-based
games in the framework are played in a similar order. The framework
provides the playing algorithmand concrete games implement the steps of
the algorithm.
The TempleMethod pattern is a solution for this problem. The usual
participants of Template Method are an abstract class declaring a final
method (the algorithm). The steps of the algorithmcan either be concrete
methods providing default behaviour or abstract methods, forcing sub-
classes to override them. GO however, does not have abstract classes nor
abstractmethods. The diagrambelow shows the structure of the Template
Method pattern as it is implemented in GO.

![](data:image/png;base64,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)

The variant steps (or hooks) of the Template Method are defined in a
Common Interface. The invariant steps are implemented in a base type,
Default Implementation. The variant steps (Primitive Operations) can be given
a default implementation in the base type. Concrete Implementations embed
the Default Implementation. Concrete Implementations have to implement
themissingmethods of the Common Interface and can override the default
behaviour.

A.3. BEHAVIORAL PATTERNS 167
Example We illustrate TemplateMethodwith a framework for turn-based
games.
Game is the Common Interface and lists the Primitive Operations.
type Game interface {
SetPlayers(int)
InitGame()
DoTurn(int)
EndOfGame() bool
PrintWinner()
}
BasicGame, theDefault Implementation, defines PlayGame, the Template
Method. PlayGame defines the algorithmgames followand calls for each
variant step a primitive operation. The additional parameter game of type
Game on PlayGame is necessary, to call “right” primitive operations. The
receiver of PlayGame is this and this is of type BasicGame. The tem-
platemethod however has to call the primitive operations of the concrete
game. Hence, calling this.EndOfGame() will result in an error, since
BasicGame does not implement thismethod.
type BasicGame struct {}
func (this *BasicGame) PlayGame(game Game,
players int) {

game.SetPlayers(players)
game.InitGame()
for !game.EndOfGame() {
game.DoTurn()
}
game.PrintWinner()
}

168 APPENDIX A. DESIGN PATTERN CATALOGUE
The Chess struct embeds BasicGame, somemethods are overridden,
some are not; the EndOfGamemethod is also supplied; therefore, Chess
implicitly implements Game. Games of chess can be played by calling
PlayGame on a Chess.
type Chess struct {
*BasicGame
...
}
func (this *Chess) SetPlayers(players int) {}
func (this *Chess) DoTurn() { ... }
func (this *Chess) EndOfGame() bool { ... }
Here is howclientswould use Chess:
chess := NewChess()
chess.PlayGame(chess, 2)
An instance of Chess is created and PlayGame called. The chess object
gets passed alongside the numbers of players.
Discussion At first glance, the GO implementation looksmuch like the
one fromDesign Patterns: a set ofmethods is defined in the Game interface,
these methods are the fine grained parts of the algorithm; a BasicGame

struct defines the static glue code which coordinates the algorithm(in the
PlayGame method), and default implementations for most methods in
Game.
Themajor difference comparedwith standard implementations is that
we must pass a Game object to the PlayGame method twice: first as the
receiver this BasicGame and then as the first argument game Game.
*
This is the client-specified self pattern[90]. BasicGame will be embedded
into concrete games (like Chess). Inside PlayGame, callsmade to thiswill
callmethods on BasicGame; GO does not dispatch back to the embedding
object. If we wish the embedding object’s methods to be called, then we
must pass in the embedding object as an extra parameter. This extra param-
etermust be passed in by the client of PlayGame, andmust be the same
object as the first receiver. Every additional parameter is places a further

A.3. BEHAVIORAL PATTERNS 169
burden onmaintainers of the code. Furthermore the parameter has the po-
tentially for confusion: chess.PlayGame(new(Monopoly), 2)). Here
we pass an object of type Monopoly to the PlayGamemethod of the object
chess. This callwill not play Chess, butMonopoly. We discuss this idiom
further in Section 2.2.2.
The combination of BasicGame and the interface Game is effectively
making EndOfGame an abstractmethod. Game requires thismethod, but
BasicGame doesn’t provide it. Embedding BasicGame in a type and not
implementing EndOfGamewill result in a compile time error,when used
instead of a Game object.
The benefits of associating the algorithmwith a type allows for provid-
ing default implementations, using of GO’s compiler to check for unim-
plementedmethods. The disadvantage being, that sub-types like Chess

could override the templatemethod, contradicting the idea of having the
templatemethod fixed and sub types implementing only the steps. In Java,
the PlayGamemethodwould be declared final so that the structure of
the game cannot be changed. This is not possible in GO because there is no
way to stop amethod being overridden.
In class-based languages, the BasicGame class would usually be de-
clared abstract, because it does notmake sense to instantiate it. GO, how-
ever, has no equivalent of abstract classes. To prevent BasicGame objects
being used,we do not implement allmethods in the Game interface. This
means that BasicGame objects cannot be used as the Client-specified self
parameter to PlayGame (because it does not implement Game).

170 APPENDIX A. DESIGN PATTERN CATALOGUE

### A.3.11 Visitor

Intent Separate the algorithmfroman object structure it operates on.
Context Consider performing an action on every part of a complex object
structure like a car, actions like printing out the parts name or checking the
parts status. Each part offers different operations. The algorithms to be
performed on the car parts are not known in advance. It is necessary to be
able to add newalgorithmswithout having to change the car or its parts.
The Visitor pattern offers a solution. The diagram below shows the
structure of the pattern.

![](data:image/png;base64,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Bw64uLh4eHiIRKJp06ap/f23f/9+Q0PDTp06iUSiwMDAx48f03LODJUFs1tH0dN7CSH29vbr1q0bO3asSCRibw2lohLN01NBLvPjx48HBgayh3VGjhwpkUguXLhA/+TxeMOHD7948aImK20Km8OWV1ZWPn78WPbXLfYfzdNTQav9hxBy8eLFUaNGqa321aGPap37GPooOdh/Gq+P4sm2FrDCw8OnTZsme3YItFrx8fGjR49OTU3V9tQcgOfPn3t6ev7999+qO5MVdbSqOTEx8eHDh3KH8KB1Qh8F9aZhHxUREREQENDyZxkBXlHPnj379ev3448/6joRaH6++uqrL7/8Ej8+oVGhj4J606qPwi8SAPX279+v6xSgWdqxY4euU4BWAX0U1I9WfRRmGQEAAABADQwZAQAAAEANDBkBAAAAQA0MGQEAAABAjZY/ZKyoqJgxY8acOXMattqioiL6TMaGrVaRVCpds2YN54qSkpLGjBnj4eHRvXv3gQMHJiQkNOraX758OWjQoHpUEh8fLxAINm7cKFc+a9as/fv379ix45tvvmmgfAHgH+ijNIc+CkATLX/IWF1dvXfv3p9++qlhq42NjfX29tbwOUivIiEh4dChQ4or+uuvv4YNG/bxxx8nJCTcv39/6tSpgYGBZWVl9VgFfei42rVbWFjI3S1WQ7Nnzx4zZsydO3dkC3fu3FlcXDx58uQJEybs2bOnHtUCgAroozSHPgpAEy1/yPjqioqKpk6d2rNnz44dO06dOlUikRBCYmJifH19lb27bNmyt99+OyQkpEOHDiEhIbdv3x4+fLiLi8v777+vok561/sxY8Z06dKlb9++JSUlSUlJQUFBOTk5Xl5eX375JZtSdXX1O++8s2nTpsDAQFoyadIkPp8fHR29atWq0NDQkJAQNzc3f3//nJwcZWtctmzZuHHjhg4d2q1bt5cvX8bExLzxxhve3t7Ozs5LliyhMwSya1+2bNmqVavow9ffe+89T0/PDh06rFy5ktavmDwtj4iIsLa2njNnTnx8PJt/RkbGmjVrNm/eTAixtraWSCTp6emvqz0BWhr0UeijAF4HpqWjT2Zs06aNVktt2rSpqKiIvh46dOjp06cZhqmtrR08ePCJEycYhhkzZszRo0eVvRsUFBQSElJVx8bGZsqUKdXV1WVlZaampi9evFC21JAhQ0aM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zp073d3dFy5cKDe+cePGDRs29OzZkxBiY2MzdepU2uCXX37ZsmVLRkaGjo7OgAEDfvjhB3d3dzoQ2Lt376ioqCVLlmRnZ7/99tvLly9nu7Wyspo/fz773BT1O9m5c6eTk1NtbS0hhD7CYNq0aT///DMhZMCAAaGhoZMmTTI2NpZcBEVxAID/lIwTJkx46623NJ0GNIKIiIiqqipNZwGgncrKyg4ePLhjx478/Pzp06cHBwfLjfv6+mZlZcndqBoZGe3Zs8fX17eysjI0NHTKlClxcXHspxEREdHR0RYWFgkJCZJT5ebmbt26lRag9erE0tJSKBTSUrKkpERH539nri9atGj37t1Lly6dOHFiaGhor169lMcBAP7jypUrDGiFPXv2PHv2TNNZALQImZmZu3btaqzeZs+ebWNjM3369IsXL9bW1iqJJyYmEkJevXqlvMNr167xeDz6OjY2lhCSkpLCfkojpnXs7OwmTZqUnp5e304k49XV1bKT5+bmbtiwwcvL64033rh06ZLK+OtISEg4ceJEo3QFAJqi/VdMAwC8puTkZAsLCx8fn+7du3M4HCVxc3NzehqijY2NVCd//vnnunXrHj58WCOBx+PRT93c3KTaFxYWSg4NNqwTJaytrX18fO7evfvXX3/l5+erjANAG9cmLn8BAHgdt2/fjoyMTEtL8/LyGj169OHDhysrK+XGbW1tHRwcrl27JtVDbm7u+PHjFy5cmJubW1JS8ueffxJCGIZhG3C5qrfGDehEssBlPXr0aPny5c7OzitWrBgyZEhmZuaUKVOUxAEAUDICAKjF29t769attIr66aef1q9fryi+YsWKZcuW0VMS8/PzDx48SAipqKgQi8VWVla6uro5OTlhYWENyKEBnVhbW9NLcySDAwcOrKioOHfu3I0bN2bPnm1kZKQ8DgDQJm7lDQDQWAwMDKbVkbrUTDKup6fH4/GmTZuWmZmpr68/e/bsqVOnuri4bNy48b333hMKhZ07d541a9bly5frO/cGdNKxY8fFixf7+/sbGBhMnjw5PDycEPL8+XM9PT3ZxoriAACEEM6VK1dwxbR2iIiIGDRokIuLi6YTAdC8rKysCxcuhISEaDoR+I/ExMTMzMygoCBNJwIADac9B6Zf83ELAAAAAKCIlpSMjfW4BQAAAACQ1TpKxrt37y5YsOCHH35QFF+5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)

Visitor (car part visitor) defines a visitmethod for each type of Concrete
Element (print visitor, check status visitor). The object that is being visited
determines which visitmethod is to be called. Concrete Visitors implement
the visit methods. The Object Structure (car) is made up of Elements. All
Elements have to define amethod accepting a visitor. Visitors can move
over the Object Structure and visit each part of it calling their Accept

A.3. BEHAVIORAL PATTERNS 171
methods. TheAcceptmethods call the visitor’s corresponding visitmethod.
Example We demonstrate the Visitor pattern with the above described
car example. A car consists of car parts. Visitors can visit the parts and
performan operation of each part.
CarPart defines the common interface for all elements that should be
visitable. CarPart is the Element interfacementioned above.
type CarPart interface {
Accept(CarPartVisitor)
}
Wheel and Engine are Concrete Elements. The Accept methods call
the according visitXXX method of the visitor and pass a reference of
themselves. The passed in visitor object will then do its functionality
with the current object.
type Wheel struct {
Name string
}
func (this *Wheel) Accept(visitor CarPartVisitor) {
visitor.visitWheel(this)
}
type Engine struct {}
func (this *Engine) Accept(visitor CarPartVisitor) {

visitor.visitEngine(this)
}
Car is the Object Structuremaintaining a list of CarPart objects. New-
Car assembles a car thiswith fourwheels and an engine.
type Car struct {
parts []CarPart
}
func NewCar() *Car {
this := new(Car)
this.parts = []CarPart{
&Wheel{"front left"},
&Wheel{"front right"},
&Wheel{"rear right"},
&Wheel{"rear left"},
&Engine{}}

172 APPENDIX A. DESIGN PATTERN CATALOGUE
return this
}
Car also implements the Accept method which iterates over the car
parts and calls their Acceptmethod. This is an example of the Composite
Pattern (see Appendix A.2.3).
func (this *Car) Accept(visitor CarPartVisitor) {
for _, part := range this.parts {
part.Accept(visitor)
}
}
CarPartVisitor is the common Visitor interface mentioned before
declaring visitXXX operations, one for each concrete element type.
type CarPartVisitor interface {
visitWheel(wheel *Wheel)
visitEngine(engine *Engine)
}
The PrintVisitor is a Concrete Visitor implementing the visitmeth-
ods declared in CarPartVisitor. Themethods print amessage on the
command line, describingwhat kind of elementwas visited.
type PrintVisitor struct{}
func (this *PrintVisitor) visitWheel(wheel *Wheel) {
fmt.Printf("Visiting the %v wheel\n", wheel.Name)
}

func (this *PrintVisitor) visitEngine(engine *Engine) {
fmt.Printf("Visiting engine\n")
}
Using a PrintVisitor to print the elements of an object structure, e.g.
a car, is straight forward:
car := NewCar()
car.Accept(new(PrintVisitor))
The car object’s Acceptmethod is calledwith an instance of Print-
Visitor. Accept loops over the car’s parts and calls their Acceptme-
thod passing on the PrintVisitor object. Each part calls the according
visitXXXmethod of the visitor.

A.3. BEHAVIORAL PATTERNS 173
Discussion In programming languages supportingmethod overloading,
like C++ and Java, everymethod listed in the Visitor interface can be called
“visit”. The distinction is made by overloading the type of the concrete
elements they are visiting. GO lacksmethod overloading. Method names
have to unique for each type. This causes some implementation overhead
since each visitmethod carries its corresponding type twice: in themethod
name and in the type of its parameter.
Adding new behaviour is comparatively easy. All that is necessary
is a type implementing the visit methods defined in CarPartVisitor. The
existing code can remain unchanged. But it is hard to add new types.
Consider adding the typeWindowas a newCarPart. The CarPartVisitor
interface has to be extendedwith a visitWindowmethod and all existing

visitors had to be changed to implement the newmethod. On the one hand
the visitor patternmakes it easy to add newfunctionality to an existing type
structure, but hard to add new types. On the other hand using embedding
makes it easy to add newtypes, but adding additional functionality is hard.
GO’s language features do not help overcoming this problem, known as
the Expression Problem[89].
Consider the implementation of the PrintVisitor below.
func PrintVisitor(part CarPart) {
switch this := part.(type) {
case *Wheel:
fmt.Printf("Visiting the %v wheel\n", this.Name)
case *Engine:
fmt.Println("Visiting engine")
case *Body:
fmt.Println("Visiting body")
}
This function does the same as the type PrintVisitor: depending on the
type of the visited car part the corresponding message is printed. Type
switches are a bad alternative to dynamic dispatch.Each element type needs

its own case statement and it is easy to forget a case. By using types, the
compiler can be used to findmissing visitmethods,which is not the case
when using functionswith type switches. Furthermore visitors can accu-

174 APPENDIX A. DESIGN PATTERN CATALOGUE
mulate state, which is hard to do for functions (see Discussion of Strategy
Appendix A.3.9). The advantage of implementing visitor with functions
is that it is easier to add a function than a type with the corresponding
methods.

# Bibliography

[1] Bossie Awards 2010: The best open source application development
software, 2010. http://www.infoworld.com/d/open-source/
bossie-awards-2010-the-best-open-source-
application-development-software-140; Accessed August–
November 2010.
[2] Effective Go, 2010. http://golang.org/doc/effective_go.
html; AccessedMarch–August 2010.
[3] Go Release History, 2010. http://golang.org/doc/devel/
release.html; AccessedMarch–November 2010.
[4] Golang.org, 2010. http://golang.org; AccessedMarch–August
2010.
[5] Golang.org FAQ, 2010. http://golang.org/doc/go_faq.html;
AccessedMarch – November 2010.
[6] Package container/Vector, 2010. http://golang.org/pkg/
container/vector/; AccessedMarch–November 2010.
[7] TIOBE programming community index, 2010. http://www.tiobe.
com/index.php/content/paperinfo/tpci/index.html; Ac-
cessedMarch–November 2010.

[8] AGERBO, E., AND CORNILS, A. Theory of Language Support for Design
Patterns. Aarhus University, Department of Computer Science, 1997.
175

176 BIBLIOGRAPHY
[9] AGERBO, E., AND CORNILS, A. Howto preserve the benefits of design
patterns. In Proceedings of the 1998 ACMSIGPLAN Conference on Object-
Oriented Programming Systems, Languages & Applications (New York,
NY, USA, 1998), vol. 33 of OOPSLA ’98, ACM, pp. 134–143.
[10] ALEXANDER, C., ISHIKAWA, S., AND SILVERSTEIN, M. A Pattern
Language: Towns, Buildings, Construction. Oxford University Press,
USA, 1977.
[11] ALPERT, S. R., BROWN, K., AND WOOLF, B. The Design Patterns
Smalltalk Companion. Addison-Wesley Longman Publishing Co., Inc.,
Boston,MA, USA, 1998.
[12] ALPHONCE, C., CASPERSEN, M., AND DECKER, A. Killer ”Killer
Examples” for Design Patterns. In Proceedings of the 38th SIGCSE
Technical Symposium on Computer Science Education (New York, NY,
USA, 2007), SIGCSE ’07, ACM, pp. 228–232.
[13] BECK, K., AND CUNNINGHAM,W. Using pattern languages for object-

oriented programs. Tech. Rep. CR-87-43, OOPSLA ’87: Workshop on
the Specification and Design for Object-Oriented Programming, Sept.
1987.
[14] BECK, K., AND JOHNSON, R. Patterns generate architectures. In Pro-
ceedings of the 8th European Conference on Object-Oriented Programming
(Bologna, Italy, July 1994), R. P.Mario Tokoro, Ed., no. 821 in Lecture
Notes in Computer Science, Springer-Verlag, pp. 139–149.
[15] BISHOP, J. C# 3.0 Design Patterns. O’ReillyMedia, Inc., 2007.
[16] BISHOP, J. Language features meet design datterns: Raising the ab-
straction bar. In Proceedings of the 2nd InternationalWorkshop on the Role
of Abstraction in Software Engineering (New York, NY, USA, 2008), ROA
’08, ACM, pp. 1–7.

BIBLIOGRAPHY 177
[17] BOSCH, J. Design patterns as language constructs. Journal of Object-
Oriented Programming 11 (1998), 18–32.
[18] BRANT, J. M. Hotdraw. Master’s thesis, University of Illinois, 1995.
[19] BREESAM, K. M. Metrics for object-oriented design focusing on class
inheritance metrics. In Proceedings of the 2nd International Conference
on Dependability of Computer Systems (Washington, DC, USA, 2007),
DEPCOS-RELCOMEX ’07, IEEE Computer Society, pp. 231–237.
[20] BU¨ NNIG, S., FORBRIG, P., LA¨MMEL, R., AND SEEMANN, N. A pro-
gramming language for design patterns. In GI-Jahrestagung (1999),
Springer-Verlag, pp. 400–409.
[21] BUSCHMANN, F., HENNEY, K., AND SCHMIDT, D. Pattern-Oriented
Software Architecture: A Pattern Language for Distributed Computing,
vol. 4 ofWiley Software Patterns. JohnWiley & Sons, 2007.

[22] BUSCHMANN, F., MEUNIER, R., ROHNERT, H., SOMMERLAD, P., AND
STAL, M. Pattern-Oriented Software Architecture: A System of Patterns,
vol. 1 ofWiley Software Patterns. JohnWiley & Sons, Chichester, UK,
1996.
[23] CARDELLI, L., AND PIKE, R. Squeak: a language for communicating
with mice. In Proceedings of the 12th Annual Conference on Computer
Graphics and Interactive Techniques (New York, NY, USA, 1985), SIG-
GRAPH ’85, ACM, pp. 199–204.
[24] CHAMBERS, C., HARRISON, B., AND VLISSIDES, J. A debate on
language and tool support for design patterns. In Proceedings of the
27th ACMSIGPLAN-SIGACT Symposium on Principles of Programming
Languages (NewYork, NY, USA, 2000), POPL ’00, ACM, pp. 277–289.
[25] CHRISTENSEN, H. B. Frameworks: Putting design patterns into per-
spective. In Proceedings of the 9th Annual SIGCSE Conference on Innova-

178 BIBLIOGRAPHY
tion and Technology in Computer Science Education (New York, NY, USA,
2004), ITiCSE ’04, ACM, pp. 142–145.
[26] COPLIEN, J. O., AND SCHMIDT, D. C., Eds. Pattern Languages of
Program Design. Addison-Wesley Professional,NewYork,NY, USA,
1995.
[27] CORP, I. Occam ProgrammingManual. Prentice Hall Trade, 1984.
[28] CUNNINGHAM,W. A CRC description ofHotDraw, 1994. http://
c2.com/doc/crc/draw.html; AccessedMarch – November 2010.
[29] DECKER, R., AND HIRSHFIELD, S. Top-down teaching: Object-
oriented programming in CS 1. In Proceedings of the Twenty-Fourth
SIGCSE Technical Symposium on Computer Science Education (New York,
NY, USA, 1993), SIGCSE ’93, ACM, pp. 270–273.
[30] DIJKSTRA, E. W. Letters to the editor: go to statement considered
harmful. Communications of the ACM11 (Mar. 1968), 147–148.
[31] DORWARD, S., PIKE, R., PRESOTTO, D., RITCHIE, D., TRICKEY, H.,

AND WINTERBOTTOM, P. Inferno. In Proceedings of the 42nd IEEE
International Computer Conference (Feb. 1997), COMPCON ’97, pp. 241
–244.
[32] DORWARD, S., PIKE, R., ANDWINTERBOTTOM, P. Programming in
Limbo. In Proceedings of the 42nd IEEE International Computer Conference
(Feb. 1997), COMPCON ’97, pp. 245 –250.
[33] FAYAD, M., AND SCHMIDT, D. C. Object-oriented application frame-
works. Communications of the ACM40 (Oct. 1997), 32–38.
[34] FOOTE, B., ROHNERT, H., AND HARRISON, N., Eds. Pattern Languages
of Program Design 4. Addison-Wesley Professional, Boston,MA, USA,
1999.

BIBLIOGRAPHY 179
[35] FREEMAN, E., FREEMAN, E., BATES, B., AND SIERRA, K. Head First
Design Patterns. O’ Reilly & Associates, Inc., 2004.
[36] FROEHLICH, G., HOOVER, H. J., LIU, L., AND SORENSON, P. Hooking
into object-oriented application frameworks. In Proceedings of the 19th
International Conference on Software Engineering (NewYork,NY, USA,
1997), ICSE ’97, ACM, pp. 491–501.
[37] GABRIEL, R. Lisp: GoodNews BadNewsHowtoWin Big. AI Expert
6, 6 (1991), 30–39.
[38] GAMMA, E. JHotDraw, 1996. http://jhotdraw.sourceforge.
net/; AccessedMarch – November 2010.
[39] GAMMA, E., HELM, R., JOHNSON, R. E., AND VLISSIDES, J. Design
Patterns: Elements of Reusable Object-Oriented Software. Addison-Wesley
Professional,Mar. 1995.
[40] GANNON, J. D. An experimental evaluation of data type conventions.
Communications of the ACM20, 8 (Aug. 1977), 584–595.

[41] GAT, E. Lisp as an alternative to Java. Intelligence 11, 4 (2000), 21–24.
[42] GIL, J., AND LORENZ, D. Design patterns and language design.
Computer 31, 3 (Mar. 1998), 118 –120.
[43] GUPTA, D. What is a good first programming language? Crossroads
10, 4 (2004), 7–7.
[44] HADJERROUIT, S. Java as first programming language: A critical
evaluation. SIGCSE Bulletin 30, 2 (1998), 43–47.
[45] HANENBERG, S. An experiment about static and dynamic type sys-
tems: Doubts about the positive impact of static type systems on
development time. In Proceedings of the ACMInternational Conference
on Object-oriented Programming, Systems, Languages, and Applications
(NewYork, NY, USA, 2010), OOPSLA ’10, ACM, pp. 22–35.

180 BIBLIOGRAPHY
[46] HARMES, R., AND DIAZ, D. Pro JavaScript Design Patterns, 1 ed.
Apress, Dec. 2007.
[47] HOARE, C. A. R. Communicating Sequential Processes. Communica-
tions of the ACM21 (Aug. 1978), 666–677.
[48] JOHNSON, R. E. Documenting frameworks using patterns. In Proceed-
ings on Object-Oriented Programming Systems, Languages, and Applica-
tions (NewYork, NY, USA, 1992), OOPSLA ’92, ACM, pp. 63–76.
[49] JOHNSON, R. E., AND BRANT, J.M. Creating tools inHotDrawby com-
position. In 13th International Conference on Technology ofObject-Oriented
Languages and Systems (Versailles, France, Europe, 1994), B.Magnus-
son, B.Meyer, J.-M. Nerson, and J.-F. Perrot, Eds., no. 13 in TOOLS ’94,
Prentice Hall, pp. 445 – 454.
[50] JOHNSON, S., AND KERNIGHAN, B. The programming language B.
Tech. Rep. 8, Bell Laboratories,Murry Hill, NewJersy, January 1973.

[51] KAISER,W. Become a programming Picassowith JHotDraw. JavaWorld
(Feb. 2001).
[52] KELLEHER, C., AND PAUSCH, R. Lowering the barriers to program-
ming: A taxonomy of programming environments and languages
for novice programmers. ACMComputing Surveys 37, 2 (June 2005),
83–137.
[53] KERNIGHAN, B. A descent into Limbo. Online White-Paper. Lucent
Technologies 12 (1996).
[54] KERNIGHAN, B. W. Why Pascal is not my favorite programming
language. Tech. Rep. 100, AT&T Bell Laboratories,Murray Hill,New
Jersey 07974, Apr. 1981.
[55] KERNIGHAN, B. W. The UNIX Programming Environment. Prentice
Hall Professional Technical Reference, 1984.

BIBLIOGRAPHY 181
[56] KIRCHER, M., AND JAIN, P. Pattern-Oriented Software Architecture:
Patterns for ResourceManagement, vol. 3 ofWiley Software Patterns. John
Wiley & Sons, 2004.
[57] KO¨ LLING,M., KOCH, B., AND ROSENBERG, J. Requirements for a first
year object-oriented teaching language. In Proceedings of the Twenty-
Sixth SIGCSE Technical Symposium on Computer Science Education (New
York, NY, USA, 1995), SIGCSE ’95, ACM, pp. 173–177.
[58] KO¨ LLING, M., AND ROSENBERG, J. Blue – a language for teaching
object-oriented programming. In Proceedings of the Twenty-Seventh
SIGCSE Technical Symposium on Computer Science Education (New York,
NY, USA, 1996), SIGCSE ’96, ACM, pp. 190–194.
[59] KO¨ LLING, M., AND ROSENBERG, J. An object-oriented programdevel-
opment environment for the first programming course. In Proceedings

of the Twenty-Seventh SIGCSE Technical Symposium on Computer Science
Education (NewYork, NY, USA, 1996), SIGCSE ’96, ACM, pp. 83–87.
[60] MANOLESCU, D., VOELTER, M., AND NOBLE, J., Eds. Pattern Lan-
guages of Program Design 5. Addison-Wesley Professional, 2005.
[61] MARTIN, R. C., RIEHLE, D., AND BUSCHMANN, F., Eds. Pattern
Languages of Program Design 3. Addison-Wesley Professional, Boston,
MA, USA, 1997.
[62] MATSUMOTO, Y. Go-Gtk library, 2010. http://mattn.github.
com/go-gtk/; AccessedMarch–June 2010.
[63] MAZAITIS, D. The object-oriented paradigm in the undergraduate
curriculum: Asurvey of implementations and issues. SIGCSE Bullettin
25 (Sept. 1993), 58–64.
[64] MCIVER, L. Evaluating languages and environments for novice pro-
grammers. In 14thWorkshop of the Psychology of Programming Interest
Group (June 2002), Brunel University, pp. 100–110.

182 BIBLIOGRAPHY
[65] METSKER, S. J., AND WAKE, W. C. Design Patterns in Java, 2. ed.
Addison-Wesley Professional, Upper Saddle River, NJ, 2006.
[66] NORVIG, P. Design patterns in dynamic programming. ObjectWorld
96 (1996). http://norvig.com/design-patterns/.
[67] OLSEN, R. Design Patterns in Ruby, 1 ed. Addison-Wesley Professional,
Dec. 2007.
[68] PARKER, K. R., OTTAWAY, T. A., CHAO, J. T., AND CHANG, J. A for-
mal language selection process for introductory programming courses.
Journal of Information Technology Education 5 (2006), 133–151.
[69] PIKE, R. Newsqueak: A language for communicatingwithmice. Tech.
Rep. 143, Bell Laboraties, Apr. 1994.
[70] PIKE, R. Go, the programming language, Oct. 2009. http:
//golang.org/doc/talks/go_talk-20091030.pdf; Accessed
May – November 2010.
[71] PIKE, R. Another Go at language design, Apr. 2010.
http://www.stanford.edu/class/ee380/Abstracts/

100428-pike-stanford.pdf; AccessedMay – November 2010.
[72] PIKE, R. Another Go at language design, July 2010.
http://assets.en.oreilly.com/1/event/45/Another%
20Go%20at%20Language%20Design%20Presentation.pdf;
Accessed July – November 2010.
[73] PIKE, R. The expressiveness of Go, October 2010. http://
golang.org/doc/ExpressivenessOfGo.pdf; AccessedOctober
– November 2010.
[74] PIKE, R. Go, July 2010. http://assets.en.oreilly.com/1/
event/45/Go%20Presentation.pdf; Accessed July –November
2010.

BIBLIOGRAPHY 183
[75] PIKE, R., PRESSOTO, D., THOMPSON, K., AND TRICKEY, H. Plan 9
fromBell Labs. Computing Systems 8 (1995), 221 – 253.
[76] PRECHELT, L. Comparing Java vs. C/C++ efficiency differences to
interpersonal differences. Communications of the ACM 42, 10 (1999),
109–112.
[77] PRECHELT, L. An empirical comparison of seven programming lan-
guages. Computer 33, 10 (Oct. 2000), 23–29.
[78] PUGH, J. R., LALONDE, W. R., AND THOMAS, D. A. Introducing
object-oriented programming into the computer science curriculum. In
Proceedings of the Eighteenth SIGCSE Technical Symposium on Computer
Science Education (NewYork,NY,USA, 1987), SIGCSE ’87,ACM, pp. 98–
102.
[79] REENSKAUG, T. Models - Views - Controllers. Tech. rep., Xerox Parc,
Dec. 1979.
[80] REENSKAUG, T. Thing -Model - View - Editor. Tech. Rep. 5, Xerox
Parc,May 1979.

[81] RIEHLE, D. Case study: The JHotDraw framework. In Framework
Design: A RoleModeling Approach. ETHZu¨ rich, 2000, ch. 8, pp. 138–158.
[82] RITCHIE, D. M. The development of the C language. In The Second
ACMSIGPLAN Conference on History of Programming Languages (New
York, NY, USA, 1993), HOPL-II, ACM, pp. 201–208.
[83] SCHMAGER, F., CAMERON, N., AND NOBLE, J. GoHotDraw: Evaluat-
ing the Go programming language with design patterns. In Evaluation
and Usability of Programming Languages and Tools (PLATEAU) 2010
(2010).

184 BIBLIOGRAPHY
[84] SCHMIDT, D. C., STAL, M., ROHNERT, H., AND BUSCHMANN, F.
Pattern-Oriented Software Architecture: Patterns for Concurrent and Net-
worked Objects, vol. 2 of Wiley Software Patterns. JohnWiley & Sons,
Chichester, UK, 2000.
[85] SHALLOWAY, A., AND TROTT, J. Design Patterns Explained: A new
Perspective on Object-Oriented Design, 2 ed. Software Patterns. Addison-
Wesley Professional, 2004.
[86] SKUBLICS, S., ANDWHITE, P. Teaching smalltalk as a first program-
ming language. In Proceedings of the Twenty-Second SIGCSE Technical
Symposium on Computer Science Education (New York, NY, USA, 1991),
SIGCSE ’91, ACM, pp. 231–234.
[87] TEMTE, M. C. Let’s begin introducing the object-oriented paradigm.
In Proceedings of the Twenty-Second SIGCSE Technical Symposium on
Computer Science Education (New York, NY, USA, 1991), SIGCSE ’91,
ACM, pp. 73–77.

[88] TICHY, W. Should computer scientists experimentmore? Computer
31, 5 (May 1998), 32–40.
[89] TORGERSEN, M. The expression problemrevisited. In ECOOP 2004 -
Object-Oriented Programming (2004), vol. 3086 of Lecture Notes in Com-
puter Science, Springer Berlin / Heidelberg, pp. 1–44.
[90] VILJAMAA, P. Client-Specified Self. In Pattern Languages of Program
Design. Addison-Wesley Publishing Co., New York, NY, USA, 1995,
ch. 26, pp. 495–504.
[91] VIRDING, R., WIKSTRO¨M, C., AND WILLIAMS, M. Concurrent Pro-
gramming in Erlang, 2 ed. PrenticeHall International (UK) Ltd.,Hert-
fordshire, UK, UK, 1996.
[92] VLISSIDES, J. Pattern Hatching: Design Patterns Applied. Addison-
Wesley Longman Ltd., Essex, UK, UK, 1998.

BIBLIOGRAPHY 185
[93] VLISSIDES, J. M., COPLIEN, J. O., AND KERTH, N. L., Eds. Pattern
Languages of Program Design 2. Addison-Wesley Professional, Boston,
MA, USA, 1996.
[94] WEINAND, A., AND GAMMA, E. ET++ – a portable, homogenous
class library and application framework. In Proceedings of the UBILAB
’94 Conference (1994), Universita¨tsverlag Konstanz.
[95] WINTERBOTTOM, P. Alef Language ReferenceManual, 2 ed. Bell Labs,
Murray Hill, 1995.
[96] WIRTH, N. The programming language Pascal. Acta Informatica 1, 1
(1971), 35–63.
[97] WIRTH, N., AND GUTKNECHT, J. Project Oberon: The Design of an Op-
erating System and Compiler. ACMPress/Addison-Wesley Publishing
Co., NewYork, NY, USA, 1992.

186 BIBLIOGRAPHY
