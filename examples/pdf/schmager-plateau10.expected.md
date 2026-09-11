# GoHotDraw: Evaluating the Go Programming

# Language with Design Patterns

## Frank Schmager Nicholas Cameron James Noble

Victoria University ofWellington, Victoria University ofWellington, Victoria University ofWellington,
New Zealand New Zealand New Zealand
frank.schmager@ecs.vuw.ac.nz ncameron@ecs.vuw.ac.nz kjx@ecs.vuw.ac.nz

## Abstract discussed requirements for programming languages for beginners

[10]. The methods and criteria proposed in the literature are usu-
Go, a new programming language backed by Google, has the po-
ally hard to measure and are prone to be subjective.
tential for widespread use: it deserves an evaluation. Design pat-
In this paper, we use design patterns to evaluate a programming
terns are records of idiomatic programming practice and inform
language. There are a number of books describing the GoF design
programmers about good program design. In this study, we evalu-
patterns in different programming languages (Smalltalk [4], Java
ate Go by implementing design patterns, and porting the “pattern-
[17], JavaScript [12], Ruby [19]). Implementations of design pat-
dense” drawing framework HotDraw into Go, producing GoHot-
terns differ due to specifics of the language used. There is a need
Draw. We show how Go’s language features affect the implemen-

for programming language specific pattern descriptions. Our use
tation of Design Patterns, identify some potential Go programming
of patterns should give programmers insight into how Go-specific
patterns, and demonstrate how studying design patterns can con-
language features are used in everyday programming, and howgaps
tribute to the evaluation of a programming language.
compared with other languages can be bridged.
Categories and Subject Descriptors D.3.0 [Programming Lan- We have implemented all 23 patterns from Design Patterns in
guages]: General Go: for space reasons we discuss only the Singleton, Adapter, and
Template Method patterns in this paper. The implementations of
General Terms Design, Languages
these patterns illuminate specific Go language features: Singleton
demonstrates howGo’s source code is structured in packages rather
Keywords Go, Design Patterns

than classes, and Go’s visibility rules; Adapter allows us to com-
pare embedding with composition in Go; and TemplateMethod il-

## 1. Introduction

lustrates the flow of control between embedding and embedded ob-
Go is a new object-oriented programming language, developed at jects. To investigate how design patterns integrate in Go, we ported
Google by Rob Pike and others. Go is used “internally at Google the core of the well-known drawing framework HotDraw into Go,
for some production stuff” [18] and has found an active community thus GoHotDraw.
since its publication in November 2009 [1]. This paper describes This paper is organised as follows: in Sect. 2 we give a brief
our experiences implementing the patterns from Design Patterns overview of Go; in Sect. 3 we present case studies implementing
[9], and the HotDraw drawing editor framework in Go.We discuss three design patterns and JHotDraw in Go; in Sect. 4 we discuss

the differences between our implementations of design patterns our experience using Go, with a focus on design patterns; in Sect. 5
in Go and in other programming languages (primarily Java) and we discuss future work; in Sect. 6 we conclude.
suggest programming idioms in Go that may develop into design
patterns.
Many researchers have proposed methods and criteria for eval-
uating programming languages [26, 22]. Direct comparisons of

## programming languages have been conducted: a comparison of 2. Background

Java with C] [5]; a comparison of FORTRAN-77, C, Pascal and
In this section we introduce the Go programming language.
Modula-2 [13]. Others investigated suitability as a first program-
ming language: Parker et al. compiled a list of criteria for introduc-
tory programming courses at universities [20]; McIver proposed
evaluating languages together with IDEs [16]; Hadjerrouit exam- 2.1 The Go Programming Language
ined Java’s suitability as a first programming language [11]; Clarke
Go is an object-oriented programming language with a C-like syn-
used questionnaires to evaluate a programming language [6];Gupta
tax. Go is designed as a systems language and has a focus on con-
currency; it is “expressive, concurrent, garbage-collected” [3]. It
supports a mixture of static and dynamic typing, and is designed to
be safe and efficient.Aprimarymotivation seems to be compilation

speed (and indeed, Go programs compile blazingly fast).
Go has objects, and structs rather than classes. Go has no con-
cept of inheritance, reuse is supported by embedding. There are no
Copyright is held by the author/owner(s). This paper was published in the proceed-
explicit subtype declarations, but in some cases, structural subtypes
ings of the Workshop on Evaluation and Usability of Programming Languages and
Tools (PLATEAU) at the ACM Onward! and SPLASH Conferences. October, 2010. are inferred. Pointers are explicit and indicated by an asterisk be-
Reno/Tahoe, Nevada, USA. fore the type. Unlike C, however, pointer arithmetic is not allowed.

Objects and methods The following code example defines the objects. On each line in the following listing, an Example object
type Car with a single field affordable with type bool: is created and a pointer to the new object stored in anExample. In
each case the fields have the following values:
type Car struct {
affordable bool Example.Name == "" and Example.aField == 0.
}
1 var anExample *Example
Following C++ rather than Java, methods in Go are defined 2 anExample = new(Example)
outside struct declarations. Multiple return values are allowed and 3 anExample = &Example {}
4 anExample = &Example {"",0}
may be named; the receiver must be explicitly named. Go does not
5 anExample = &Example{Name:"", aField :0}
support overloading or multi-methods.
The following listing defines the method Refuel for Car ob-
jects:
Packages Go source code is structured in packages. Go provides

func (this *Car) Refuel(liter int) (price float) {...} two scopes of visibility: members beginning with a lower case
letter (e.g., affordable) are only accessible inside their package;
Embedding Reuse in Go is supported by embedding. If amethod members beginningwith an upper case letter (e.g., Car, Refuel())
or field cannot be found in an object’s type definition, then em- are publicly visible.
bedded types are searched, and method calls are forwarded to the
embedded object. This is similar to subclassing, the difference in

## 3. Case Studies

semantics is that an embedded object is a distinct object, and fur-
ther dispatch operates on the embedded object, not the embedding We have implemented all 23 GoF design patterns in Go. In this
one. For example, the following code shows a Truck type which section, we describe our experience in implementing three of them:
embeds the Car type and, therefore, gains the method Refuel: Singleton, Adaptor, and TemplateMethod.
type Truck struct {
Car 3.1 Singleton
affordable bool
} The Singleton design pattern [9] ensures that only a single instance
of a type exists in a program, and provides a global access point to
Objects can be deeply embedded andmultiply embedded.Name
that object.
conflicts are only a problem if the ambiguous element is accessed.
In Java, Singletons are implemented with static fields. In Go,
there are no static class members, so instead we use Go’s package

Interfaces Interfaces in Go are abstract representations of be-
access mechanisms and functions to provide similar functionality.
haviour — sets of methods. The Go compiler infers if an object
For example, we may wish to have only a single registry object
satisfies an interface. This part of the type system is structural: if
in a program:
an object implements the methods of an interface, then it has that
interface as a type. No annotation by the programmer is required.
package reg
Interfaces are allowed to embed other interfaces: the embedding
interface will contain all the methods of the embedded interfaces. type Registry interface {
There is no explicit subtyping between interfaces, but since type DoSomething ()
}
checking for interfaces is structural and implicit, if an object im-
plements an embedding interface, then it will always implement
type registry struct {

any embedded interfaces. Every object implements the empty in- ...
terface interface; other, user-defined empty interfaces may also }
be defined. The listing below defines Refuelable as an interface
var theRegistry Registry
with a single method Refuel().
func GetRegistry () Registry {
type Refuelable interface {
if theRegistry == nil {
Refuel(int) float
theRegistry = &registry {}
}
}
return theRegistry
Both Car and Truck implement this interface, even though it
}
is not listed in their definitions (and indeed, Refuelable may
have been defined long afterwards). Type checking of non-interface func (this *registry) DoSomething () { ... }
types is not structural: a Truck cannot be used where a Car is
expected. The variable theRegistry is a reference to the only instantia-
tion of registry. The struct registry cannot be named outside

Functions and goroutines Go supports functions (methods with
the reg package, so it can only be instantiated inside the package.
no receiver) as well as function pointers and closures. Goroutines
The public function GetRegistry provides access to the singleton
are functions that execute in parallel with other goroutines. Gorou-
registry, creating a new object if none exists when it is called.
tines are designed to be lightweight and to hide many of the com-
The object is provided via the interface Registry so that it can
plexities of thread creation and management. The keyword ‘go’ in
be used and stored outside the package. Variables with interface
front of a function call executes the function in its own goroutine.
type are always passed by reference, so copies of theRegistry
Goroutines can communicate (asynchronously) with other gorou-

will not be made when calling functions or methods. Registry is
tines through channels.
a non-empty interface, so it is unlikely to be implemented by acci-
Object initialisation Go objects can be created fromstruct defini- dent (see Sect. 4.1); other objects implementing Registry can be
tions with or without the new keyword. Fields can be initialised by created (as in other implementations of Singleton). Our implemen-
the user when the object is initialised, but there are no constructors. tation does not prevent multiple instantiations of registry being
Newly created objects inGo are always initialisedwith a default created inside the reg package; in particular, this might be done
value (nil, 0, false, "", etc.). There are multiple ways to create inadvertently by embedding registry.

3.2 Adapter func (this *BasicGame) PrintWinner () {
fmt.Println ("A Player won")
The Adapter design pattern adapts the interface of an object into }
a different interface to make otherwise incompatible objects work
Below is a concrete game of chess. The Chess struct em-
together.
beds BasicGame, some methods are overridden, some are not; the
In the following example, we will adapt a Reptile object to
EndOfGame method is also supplied; therefore, Chess implicitly
the Animal interface; the Slither method of Reptile provides
implements Game and games of chess can be played by calling
the functionality found in Animal’s Move.
PlayGame on a Chess object.
package animals
type Chess struct {
type Animal interface { *BasicGame
Move() ...
} }
type Cat struct {} func (this *Chess) SetPlayers(players int) {}
func (this *Cat) Move() { ... } func (this *Chess) DoTurn () { ... }

func (this *Chess) EndOfGame () bool { ... }
package reptiles
The major difference compared with standard implementations
is that we must pass a game object to the PlayGame method
type Reptile struct {}
twice: first as the receiver (this *BasicGame) and then as the
func (this *Reptile) Slither () { ... } first argument (game Game). This is the client-specified self pat-
tern [25]. BasicGame will be embedded into concrete games (like
The struct ReptileAdapter adapts an embedded reptile so that
Chess). Inside PlayGame, calls made to this will call methods on
it can be used as an Animal. ReptileAdapter objects implicitly
BasicGame; Go does not dispatch back to the embedding object.
implement the Animal interface.
If we wish the embedding object’s methods to be called, then we
package client must pass in the embedding object as an extra parameter. This ex-

tra parameter must be passed in by the client of PlayGame, and
type ReptileAdapter struct {
must be the same object as the first receiver.We discuss this idiom
*reptiles.Reptile
} further in Sect. 4.1.
In class-based languages, the BasicGame class would usually
func (this *ReptileAdapter) Move() { Slither () }
be declared abstract, because it does not make sense to instantiate
it. Go, however, has no equivalent of abstract classes. To prevent
Alternatively, we could use composition rather than embedding
BasicGame objects being used, we do not implement all methods
in our adapter object. This is a similar situation to class-based
in the Game interface. This means that BasicGame objects cannot
languageswhere an adapter can use inheritance or composition.We
be used as the client-specified self parameter to PlayGame (because
discuss composition vs. embedding as a design choice in Sect. 4.

it does not implement Game). We cannot prevent BasicGame ever
3.3 TemplateMethod being instantiated, but we can prevent it being used within this
code.
The TemplateMethod design pattern can be used when the outline
In Java, the PlayGame method would be declared final so that
of an algorithm should be invariant between classes, but individual
the structure of the game cannot be changed. This is not possible in
steps may vary.
Go because there is no way to stop a method being overridden.
We illustrate Template Method in Go with a framework for
turn-based games. At first glance, the Go implementation looks
3.4 GoHotDraw
much like the standard one: a set of methods is defined in the
HotDraw is a framework for drawing editors and graphical appli-
Game interface, these methods are the fine grained parts of the
cations [14]. As a larger case study, we ported the core of HotDraw

algorithm; a BasicGame struct defines the static glue code which
to Go, thus GoHotDraw.We chose to port HotDraw because its de-
coordinates the algorithm (in the PlayGame method), and default
sign is well known, and because it makes extensive use of design
implementations for most methods in Game.
patterns [7, 23, 15].We based our port on the the source code and
type Game interface {
documentation of JHotDraw 5.3 and 7.2 [8].
SetPlayers(int)
InitGame () Because JHotDraw is quite large, we concentrated on the
DoTurn(int) essence of the framework (Drawings contain Views, Views con-
EndOfGame () bool tain Figures, Editors enable Tools to manipulate Drawings) and
PrintWinner ()
implemented only a subset of HotDraw’s features — we support
}
only rectangle Figures, but those figures can be selected, and then

type BasicGame struct {} moved, and resized via handles. This subset is sufficient to cover all
the key patterns in HotDraw’s design. As a guide, JHotDraw v5.3
func (this *BasicGame) PlayGame(game Game ,
is about 15000 lines of code and v7.2 about 70000 lines, including
players int) {
game.SetPlayers(players) several complete applications. Our GoHotDraw subset is around
game.InitGame () 2000 lines.
for !game.EndOfGame () {
GoHotDraw’s design is very similar to the Smalltalk and Java
game.DoTurn ()
versions of HotDraw. GoHotDraw uses many patterns, and gener-
}
game.PrintWinner () ally they work as well in Go as in other languages. For example,
} the Composite pattern supports composite Figures; the Observer
pattern notifies Views of changes in the Figures they display; the
func (this *BasicGame) SetPlayers(players int) {}

Chain of Responsibility pattern manipulates Figures via Handles;
func (this *BasicGame) InitGame () {}
func (this *BasicGame) DoTurn () {} theMediator pattern couples Views and Tools via DrawingEditors;
the Strategy pattern supports multiple repainting algorithms; the

## Adapter pattern couples GoHotDraw to one of Go’s underlying 4. Discussion

graphics libraries (XGB); the Null Object pattern [27] introduces
In this section, we reflect on what we have learnt about design
NullHandles and NullTools to avoid handling null objects.
patterns in Go: both how existing patterns are implemented in Go,
As a result, GoHotDraw’s structs and embedding generally par-
and what new, Go-specific patterns we may have uncovered.
allel JHotDraw’s classes and inheritance, and GoHotDraw’s inter-
Rob Pike has described design patterns as “add-on models” for
faces are generally similar to those in the Java versions—perhaps
languages whose “standard model is oversold” [21]. We disagree:
for this reason, they tend to declare more than just the one or two
patterns are a valuable tool for helping to design software that is
methods usually preferred in Go [21].
easy to maintain and extend. Go’s language features have not re-

The key difference between the Go and Java HotDraw designs
placed design patterns: we found that only Adapter was signifi-
comes from the difference between Go’s embedding and Java’s
cantly simpler inGo than Java, and some patterns, such as Template
inheritance (described in Sect. 3.3): methods on “inner” embedded
Method, seemto bemore difficult. In general,wewere surprised by
structs in Go cannot call back to methods in “outer” embedding
how similar the Go pattern implementations were to implementa-
objects. In contrast, Java super class methods most certainly can
tions in other languages such as C++ and Java.
call “down” to methods defined in their subclasses, and this is the
As well as design patterns, the Gang of Four [9] suggest some
key to the template method pattern.
general design principles:
As a result, GoHotDraw’s Figure interface — the central in-

terface of HotDraw—is significantly different to the Java version. “Program to an interface, not an implementation” Go helps
While both interfaces provide the samemethods,many (if notmost) programmers to follow this advice. Go’s syntax for interfaces is
of thosemethods have to have an additional Figure parameter, used concise and straightforward. Types implicitly implement inter-
as a client-specified self to support template methods. faces; there is no additional syntactic overhead. On the other hand,
To take one simple example: a figure is empty if its size is Go does not have abstract classes. Go can simulate abstract meth-
smaller than 3-by-3 pixels. This method is defined for DefaultFig- ods however, as discussed below.
ure, and calls the GetSize method defined in the Figure interface:
“Favour object composition over class inheritance” Go has no

func (this *DefaultFigure) IsEmpty(fig Figure) bool { inheritance. Reuse can be achieved through either language-level
dimension := fig.GetSize(fig)
embedding or program-level composition. Go thus favours compo-
return dimension.Width < 3 || dimension.Height < 3
} sition over inheritance. The GoF’s experience was that designers
overused inheritance. Since embedding is an automated form of
The problem is that this method needs to call GetSize on the
composition, it is not obvious whether the same will apply to em-
correct “substructure” (aka subclass): in HotDraw, a RectangleFig-
bedding vis-a`-vis composition. Embedding has many of the draw-
ure inherits from DefaultFigure and thus inherits a suitable defini-
backs of inheritance: it affects the public interface of objects, it
tion for GetSize (as well as other methods). In Java, DefaultFigure

is not fine-grained (i.e, no method-level control over embedding),
could simply call this.GetSize() and the call will be dynami-
methods of embedded objects cannot be hidden, and it is static.
cally dispatched and run the correct method. In Go, this call will
try to invoke the (non-existent) GetSize method on DefaultFigure: 4.1 Go idioms
a client-specified self is needed for dynamic dispatch.We would be
We found that programming in Go required some idiomatic pro-
interested to see if there were a more Go-flavoured way to imple-
gramming practices. It is probably premature to call these design
ment this functionality.
patterns: there may be better, more Go-specific ways of addressing
This problem is exacerbated when a design needs multiple lev-
the problems, or they may be found to be indicative of bad practice
els of embedding or inheritance. Following JHotDraw, GoHot-
rather than good.

Draw’s DefaultFigure is embedded in CompositeFigure; Compos-
iteFigure is embedded in Drawing; Drawing is then further embed- Client-specified self Embedding does not support all the usual
ded in StandardDrawing. These multiple embeddings mean many object-oriented behaviour of dynamic dispatch on the self/this ob-
Figure methods require a client-specified self parameter to work ject. In particular, Go will dispatch from outer objects to inner ob-
correctly, as the final version of the Figure interface illustrates. jects (up fromsubclasses to superclasses) but not in the other direc-
Of 21 methods in that interface, six require the additional client- tion, from inner objects to outer objects (down from superclasses
specified self argument (highlighted in bold): to subclasses). Downwards dispatch is often useful in general, and

particularly so in the Template Method and Factory Method pat-
type Figure interface {
MoveBy(figure Figure, dx int , dy int) terns. Downwards dispatch can be emulated by passing the outer-
basicMoveBy(dx int , dy int) most “self” object as an extra parameter to all methods that need
changed(figure Figure)
it—implementing the client-specified self pattern [25]. To use dy-
GetDisplayBox () *Rectangle
namic dispatch, the method’s receiver must also still be supplied.
GetSize(figure Figure) *Dimension
IsEmpty(figure Figure) bool Using client-specified self is less satisfactory than proper inher-
Includes(other Figure) bool itance: it requires collaboration of an object’s client to work; the
Draw(g Graphics)
invariant that the self parameter is in fact the self is not enforced
GetHandles () *Set
by the compiler, and there is scope for error if an object other than

GetFigures () *Set
SetDisplayBoxRect(figure Figure, rect *Rectangle) the correct one is passed in. The extra parameter complicates the
SetDisplayBox(figure Figure, topLeft , bottomRight *Point) code making it harder to read and write, for no clear benefit over
setBasicDisplayBox(topLeft , bottomRight *Point)
inheritance.
GetListeners () *Set
AddFigureListener(l FigureListener)
Abstract classes Go has no equivalent of abstract classes (classes
RemoveFigureListener(l FigureListener)
which are partially implemented and cannot be instantiated). Ab-
Release ()
GetZValue () int stract classes are commonly used both in design patterns and
SetZValue(zValue int) object-oriented programming in general. Interfaces can be used
Clone() Figure
to definemethods without implementations, but these cannot easily
Contains(point *Point) bool
be combined with partial implementations. As illustrated by our
}

discussion of the Template Method pattern (Sect. 3.3), however,

Go’s implicit interface declarations provide a partial work around: ² The initial-capital-for-public encapsulation scheme is nice to
concrete methods are provided in a base-struct and an interface write, but hard to read, and making mistakes is easy. Especially
is provided which is a superset of these methods. The difference becausemany other languages use the incompatible lower-case-
between the two are the abstract methods (C++’s pure virtual or for-values, inital-capital-for-types convention.
Smalltalk’s subclassResponsibility). The interface is then used as
² We found Go’s multifarious object creation syntax — some
the type of the client-specified self parameter. This idiom is defi-
with and some without the new keyword—hard to interpret.
cient because the base object can still be instantiated, and the idiom
²

relies on clients obeying the client-specified self protocol. Defining methods outside classes, and having to specify the
receiver’s name and type explicitly is repetitive, and makes
Multiple constructors Go does not support constructors. If ini- methods hard to find, and hard to distinguish from functions
tialisation code is required for an object, then the recommended in the same package.
solution is to use a simple kind of factory method [2]. The Factory
Method pattern [9] (or at least a simplification of it) is thus regu-

## 5. FutureWork

larly used in Go programs. Go does not support overloading, and
so if multiple ‘constructors’ are required, then the factory meth- We have attempted to evaluate Go using design patterns. Any such
ods must have different names. We found that this was common evaluation is dependent on the design patterns used. The Gang
and that having to use different names was inconvenient — there of Four patterns are general-purpose programming patterns, and
is no naming convention for multiple different factory methods, so so our evaluation is of Go as a general-purpose language. Go
clients wishing to instantiate an object must either guess or check is specifically targeted at the systems and concurrent domains.
the documentation. Our most immediate future work is thus to evaluate Go using
concurrency and networking patterns [24], and patterns for systems

Comma OK Methods in Go can return multiple values. This
programming.
facility is often used to return an error signal: one return value is
Our evaluation has so far been qualitative. We would like to
the result and the other is used to signal an error. This “Comma
extend our study into quantitative evaluation by applying metrics
OK” idiom is encouraged by the Go authors [2] and used in the
to existingGo source code to understand howGo language features
libraries; unsurprisingly, it is hard to avoid. An advantage of this
are used ‘in the wild’. Such a study would currently be hampered
idiomis thatGo’s exception handlingmechanismis rarely used and
by the small volume of Go source code, but this situation is rapidly
programs are not littered with try...catch blocks, which harm
improving.
readability. On the other hand, error conditions are easier to ignore

because error checking is not enforced by the compiler.

## 6. Conclusion

Not-quite-a-marker interface Parameters in Go are often given
empty interface types which indicate the kind of object expected In this paper we have introduced Go, and evaluated it using design
by themethod, rather than any expected functionality. This idiomis patterns. Our implementations of design patterns have highlighted
common in other languages, including Java: e.g., Cloneable in the Go-specific features including embedding and interface inference.
standard library. In Go, however, all objects implicitly implement Embedding allows for an easy implementation of the Adapter pat-
empty interfaces, so using an empty interface does not help the tern, but canmake flow of controlmore complicated, as seen in our
compiler check the programmers intent. An alternative solution is implementation of TemplateMethod.

to use interfaces with a single, or very small number of, methods. Go is a language which aims to do things differently, adopting
This lessens the likelihood of objects implementing the interface a new object model that is significantly different from most object-
by accident — but does not remove it. We found this idiom used oriented languages. At least for classical object-oriented programs,
in the standard library (“In Go, interfaces are usually small: one such as drawing editors and frameworks, designs in Go do not
or two or even zero methods.” [21]) and used it frequently in our seem to be significantly different to designs in Java or C++. In
own code. Unlike much Go code, however, the key interfaces in some circumstances, the differences between embedding and in-

GoHotDraw have several tens ofmethods, closer to design practice heritance can make some patterns more difficult to implement, but
in other object-oriented languages. Go-specific idioms (or patterns) can resolve most of these difficul-
ties.
4.2 Syntax
The Go syntax is an improvement over languages such as Java

## Acknowledgments

or C++ (e.g., it supports built in slices and maps). However, it is
This work was funded in part by a Build IT Postdoctoral Fellow-
more verbose and less elegant than othermodern languages such as
ship.
Haskell and Python. For example,
² Not requiring semicolons is nice, but requiring braces, is

## References

clumsy e.g. compared to Haskell’s layout syntax (though this
is a somewhat personal preference). Forcing the programmer to [1] Golang.org Community, 2010. http://golang.org/doc/
community.html; AccessedMarch – November 2010.
put braces only on particular lines seems indefensibly inconve-
nient. [2] Effective Go, 2010. http://golang.org/doc/effective_go.
html; AccessedMarch–August 2010.
² Interface types are implicitly references, but other types must
[3] Golang.org, 2010. http://golang.org; Accessed March–August
be explicitly marked as pointers; we found this inconsistency
2010.
tripped us up repeatedly. There is no convention for distinguish-
ing interface names from type names. It is therefore hard to [4] Sherman R. Alpert, Kyle Brown, and BobbyWoolf. The Design Pat-

interpret interface declarations without knowing which other terns Smalltalk Companion. Addison-Wesley Longman Publishing
Co., Inc., Boston,MA, USA, 1998.
types are interfaces (and therefore always passed by reference)
or structs (always passed by value, and thus should often be [5] Shyamal Suhana Chandra and Kailash Chandra. A Comparison of
pointer types). Java and C]. J. Comput. Small Coll., 20(3):238–254, 2005.

[6] Steven Clarke. Evaluating a new programming language. In 13th
Workshop of the Psychology of Programming Interest Group, pages
275–289, 2001.
[7] Ward Cunningham. A CRC Description of HotDraw. http://c2.
com/doc/crc/draw.html, 1994. Retrieved 15/07/2010.
[8] Erich Gamma. JHotDraw. http://jhotdraw.sourceforge.
net/, 1996. Retrieved 15/07/2010.
[9] Erich Gamma, Richard Helm, Ralph E. Johnson, and John Vlissides.
Design Patterns. Elements of Reusable Object-Oriented Software.
Addison-Wesley,March 1995.
[10] Diwaker Gupta. What is a good first programming language? Cross-
roads, 10(4):7–7, 2004.
[11] Said Hadjerrouit. Java as first programming language: a critical
evaluation. SIGCSE Bull., 30(2):43–47, 1998.
[12] Ross Harmes and Dustin Diaz. Pro JavaScript Design Patterns.
Apress, 1 edition, December 2007.
[13] Neal M. Holtz and William J. Rasdorf. An evaluation of program-

ming languages and language features for engineering software de-
velopment. Engineering with Computers, 3(4):183–199, December
1988.
[14] Ralph E. Johnson. Documenting frameworks using patterns. InOOP-
SLA ’92: Conference Proceedings on Object-Oriented Programming
Systems, Languages, and Applications, pages 63–76, New York, NY,
USA, 1992. ACM.
[15] Wolfram Kaiser. Become a programming Picasso with JHotDraw.
JavaWorld, February 2001.
[16] Linda McIver. Evaluating languages and environments for novice
programmers. In 14th Workshop of the Psychology of Programming
Interest Group, pages 100–110. Brunel University, June 2002.
[17] Steven J. Metsker and William C. Wake. Design Patterns in Java.
Addison-Wesley, Upper Saddle River, NJ, 2. edition, 2006.
[18] CadeMetz. Google programming Frankenstein is a Go. The Register,
May 2010.

[19] Russ Olsen. Design Patterns in Ruby. Addison-Wesley Professional,
1 edition, December 2007.
[20] Kevin R. Parker, Thomas A. Ottaway, Joseph T. Chao, and Jane
Chang. A Formal Language Selection Process for Introductory Pro-
gramming Courses. Journal of Information Technology Education,
5:133–151, 2006.
[21] Rob Pike. Another go at language design. http://www.stanford.
edu/class/ee380/Abstracts/100428-pike-stanford.pdf,
April 2010. Retrieved 15/07/2010.
[22] TerrenceWPratt andMarvinVZelkowitz. Programming Languages:
Design and Implementation. Pearson Education, Inc., 4 edition, 2001.
[23] Dirk Riehle. Case Study: The JHotDraw Framework. In Framework
Design: A Role Modeling Approach, chapter 8, pages 138–158. ETH
Zrich, 2000.
[24] Douglas C. Schmidt, Michael Stal, Hans Rohnert, and Frank Busch-
mann. Pattern-Oriented Software Architecture, Volume 2: Patterns for

Concurrent and Networked Objects. Wiley, Chichester, UK, 2000.
[25] Panu Viljamaa. Client-specified self. In Pattern Languages of Pro-
gramDesign, chapter 26, pages 495–504.Addison-Wesley Publishing
Co., New York, NY, USA, 1995.
[26] N. Wirth. Programming languages: What to demand and how to
assess them. In Symposium on Software Engineering. Belfast, April
1976.
[27] BobbyWoolf. Null object. In Pattern Languages of Program Design
3, chapter 1, pages 5–18. Addison-Wesley Longman Publishing Co.,
Inc., 1997.
