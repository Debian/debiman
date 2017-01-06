package main

// Generated by "go run gentmpl.go header footer style manpage manpageerror contents pkgindex index faq".
// Do not edit manually.

var headerContent = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>{{ .Title }} — Debian Manpages</title>
<style type="text/css">
{{ template "style" }}
</style>
<link rel="search" type="application/opensearchdescription+xml" href="/opensearch.xml">
</head>
<body>
<div id="header">
   <div id="upperheader">
   <div id="logo">
  <a href="./" title="Debian Home"><img src="/openlogo-50.svg" alt="Debian" width="50" height="61"></a>
  </div>
  <p class="section"><a href="/">MANPAGES</a></p>
  <div id="searchbox">
    <form action="/jump" method="get">
      <input type="text" name="q" placeholder="manpage name">
      <input type="submit" value="Jump">
    </form>
  </div>
 </div>
<div id="navbar">
<p class="hidecss"><a href="#content">Skip Quicknav</a></p>
<ul>
   <li><a href="/">Index</a></li>
   <li><a href="/about.html">About Manpages</a></li>
   <li><a href="/faq.html">FAQ</a></li>
</ul>
</div>
   <p id="breadcrumbs">&nbsp;
     {{- range $i, $b := .Breadcrumbs }}
     {{ if eq $b.Link "" }}
     &#x2F; {{ $b.Text }}
     {{ else }}
     &#x2F; <a href="{{ $b.Link }}">{{ $b.Text }}</a>
     {{ end }}
     {{ end -}}
   </p>
</div>
<div id="content">
`
var footerContent = ``
var styleContent = `@font-face {
  font-family: 'Inconsolata';
  src: local('Inconsolata'), url(/Inconsolata.woff2) format('woff2'), url(/Inconsolata.woff) format('woff');
}

@font-face {
  font-family: 'Roboto';
  font-style: normal;
  font-weight: 400;
  src: local('Roboto'), local('Roboto Regular'), local('Roboto-Regular'), url(/Roboto-Regular.woff2) format('woff2'), url(/Roboto-Regular.woff) format('woff');
}

@font-face {
  font-family: 'Roboto';
  font-style: normal;
  font-weight: 700;
  /* TODO: is local('Roboto Bold') really correct? */
  src: local('Roboto Bold'), local('Roboto-Bold'), url(/Roboto-Bold.woff2) format('woff2'), url(/Roboto-Bold.woff) format('woff');
}

body {
	background-image: linear-gradient(to bottom, #d7d9e2, #fff 70px);
	background-position: 0 0;
	background-repeat: repeat-x;
	font-family: 'Roboto', sans-serif;
	font-size: 100%;
	line-height: 1.5;
	letter-spacing: 0.15px;
	margin: 0;
	padding: 0;
}

#header {
	padding: 0 10px 0 52px;
}

#logo {
	position: absolute;
	top: 0;
	left: 0;
	border-left: 1px solid transparent;
	border-right: 1px solid transparent;
	border-bottom: 1px solid transparent;
	width: 50px;
	height: 5.07em;
	min-height: 65px;
}

#logo a {
	display: block;
	height: 100%;
}

#logo img {
	margin-top: 5px;
	position: absolute;
	bottom: 0.3em;
	overflow: auto;
	border: 0;
}

p.section {
	margin: 0;
	padding: 0 5px 0 5px;
	font-family: monospace;
	font-size: 13px;
	line-height: 16px;
	color: white;
	letter-spacing: 0.08em;
	position: absolute;
	top: 0px;
	left: 52px;
	background-color: #c70036;
}

p.section a {
	color: white;
	text-decoration: none;
}

.hidecss {
	display: none;
}

#searchbox {
	text-align:left;
	line-height: 1;
	margin: 0 10px 0 0.5em;
	padding: 1px 0 1px 0;
	position: absolute;
	top: 0;
	right: 0;
	font-size: .75em;
}

#navbar {
	border-bottom: 1px solid #c70036;
}

#navbar ul {
	margin: 0;
	padding: 0;
	overflow: hidden;
}

#navbar li {
	list-style: none;
	float: left;
}

#navbar a {
	display: block;
	padding: 1.75em .5em .25em .5em;
	color: #0035c7;
	text-decoration: none;
	border-left: 1px solid transparent;
	border-right: 1px solid transparent;
}

#navbar a:hover
, #navbar a:visited:hover {
	background-color: #f5f6f7;
	border-left: 1px solid  #d2d3d7;
	border-right: 1px solid #d2d3d7;
	text-decoration: underline;
}

a:link {
	color: #0035c7;
}

a:visited {
	color: #54638c;
}

#breadcrumbs {
	line-height: 2;
	min-height: 20px;
	margin: 0;
	padding: 0;
	font-size: 0.75em;
	background-color: #f5f6f7;
	border-bottom: 1px solid #d2d3d7;
}

#breadcrumbs:before {
	margin-left: 0.5em;
	margin-right: 0.5em;
}

#content {
	margin: 0 10px 0 52px;
}

.mandoc {
        font-family: 'Inconsolata', monospace;
}

#footer {
	border: 1px solid #dfdfe0;
	border-left: 0;
	border-right: 0;
	background-color: #f5f6f7;
	padding: 1em;
	margin: 0 10px 0 52px;
	font-size: 0.75em;
	line-height: 1.5em;
}

hr {
	border-top: 1px solid #d2d3d7;
	border-bottom: 1px solid white;
	border-left: 0;
	border-right: 0;
	margin: 1.4375em 0 1.5em 0;
	height: 0;
	background-color: #bbb;
}

#content p {
    padding-left: 1em;
}

/* from tracker.debian.org */

a, a:hover, a:focus {
    color: #0530D7;
    text-decoration: none;
}

/* Panel styles */
.panel {
  padding: 15px;
  margin-bottom: 20px;
  background-color: #ffffff;
  border: 1px solid #dddddd;
  border-radius: 4px;
  -webkit-box-shadow: 0 1px 1px rgba(0, 0, 0, 0.05);
          box-shadow: 0 1px 1px rgba(0, 0, 0, 0.05);
}

.panel-heading {
  padding: 5px 5px;
  margin: -15px -15px 0px;
  font-size: 17.5px;
  font-weight: 500;
  color: #ffffff;
  background-color: #d70751;
  border-bottom: 1px solid #dddddd;
  border-top-right-radius: 3px;
  border-top-left-radius: 3px;
}

.panel-heading a {
    color: white;
}

.panel-heading a:hover {
    color: blue;
}

.panel-footer {
  padding: 5px 5px;
  margin: 15px -15px -15px;
  background-color: #f5f5f5;
  border-top: 1px solid #dddddd;
  border-bottom-right-radius: 3px;
  border-bottom-left-radius: 3px;
}
.panel-info {
  border-color: #bce8f1;
}

.panel-info .panel-heading {
  color: #3a87ad;
  background-color: #d9edf7;
  border-color: #bce8f1;
}


.list-group {
  padding-left: 0;
  margin-bottom: 20px;
  background-color: #ffffff;
}

.list-group-item {
  position: relative;
  display: block;
  padding: 5px 5px 5px 5px;
  margin-bottom: -1px;
  border: 1px solid #dddddd;
}

.list-group-item > .list-item-key {
  min-width: 27%;
  display: inline-block;
}
.list-group-item > .list-item-key.versions-repository {
  min-width: 40%;
}
.list-group-item > .list-item-key.versioned-links-version {
  min-width: 40%
}


.versioned-links-icon {
  margin-right: 2px;
}
.versioned-links-icon a {
  color: black;
}
.versioned-links-icon a:hover {
  color: blue;
}
.versioned-links-icon-inactive {
  opacity: 0.5;
}

.list-group-item:first-child {
  border-top-right-radius: 4px;
  border-top-left-radius: 4px;
}

.list-group-item:last-child {
  margin-bottom: 0;
  border-bottom-right-radius: 4px;
  border-bottom-left-radius: 4px;
}

.list-group-item-heading {
  margin-top: 0;
  margin-bottom: 5px;
}

.list-group-item-text {
  margin-bottom: 0;
  line-height: 1.3;
}

.list-group-item a:hover,
.list-group-item a:focus {
  text-decoration: none;
  background-color: #f5f5f5;
}

.list-group-item.active a {
  z-index: 2;
  color: #ffffff;
  background-color: #428bca;
  border-color: #428bca;
}

.list-group-flush {
  margin: 15px -15px -15px;
}
.panel .list-group-flush {
  margin-top: -1px;
}

.list-group-flush .list-group-item {
  border-width: 1px 0;
}

.list-group-flush .list-group-item:first-child {
  border-top-right-radius: 0;
  border-top-left-radius: 0;
}

.list-group-flush .list-group-item:last-child {
  border-bottom: 0;
}

/* end of tracker.debian.org */

.panel {
float: right;
clear: right;
font-family: 'Roboto';
min-width: 200px;
}

.pkgversion {
float: right;
}`
var manpageContent = `{{ template "header" . }}

<div class="panel" role="complementary">
<div class="panel-heading" role="heading">
links
</div>
<div class="panel-body">
<ul class="list-group list-group-flush">
<li class="list-group-item">
<a href="/{{ .Meta.PermaLink }}">permalink</a>
</li>
<li class="list-group-item">
<a href="https://tracker.debian.org/pkg/{{ .Meta.Package.Binarypkg }}">package tracker</a>
</li>
<li class="list-group-item">
<a href="/{{ .Meta.RawPath }}">raw man page</a>
</li>
</ul>
</div>
</div>

<div class="panel" role="complementary">
<div class="panel-heading" role="heading">
other versions
</div>
<div class="panel-body">
<ul class="list-group list-group-flush">
{{ range $idx, $man := .Suites }}
<li class="list-group-item
{{- if eq $man.Package.Suite $.Meta.Package.Suite }} active{{- end -}}
">
<a href="/{{ $man.ServingPath }}.html">{{ $man.Package.Suite }}</a> <span class="pkgversion">{{ $man.Package.Version }}</span>
</li>
{{ end }}
</ul>
</div>
</div>

{{ if gt (len .Langs) 1 }}
<div class="panel" role="complementary">
<div class="panel-heading" role="heading">
other languages
</div>
<div class="panel-body">
<ul class="list-group list-group-flush">
{{ range $idx, $man := .Langs }}
<li class="list-group-item
{{- if eq $man.Language $.Meta.Language }} active{{- end -}}
">
<a href="/{{ $man.ServingPath }}.html">{{ DisplayLang $man.LanguageTag }}</a>
</li>
{{ end }}
</ul>
</div>
</div>
{{ end }}

{{ if gt (len .Sections) 1 }}
<div class="panel" role="complementary">
<div class="panel-heading" role="heading">
other sections
</div>
<div class="panel-body">
<ul class="list-group list-group-flush">
{{ range $idx, $man := .Sections }}
<li class="list-group-item
{{- if eq $man.Section $.Meta.Section }} active{{- end -}}
">
<a href="/{{ $man.ServingPath }}.html">{{ $man.Section }} (<span title="{{ LongSection $man.Section }}">{{ ShortSection $man.Section }}</span>)</a>
</li>
{{ end }}
</ul>
</div>
</div>
{{ end }}

{{ if gt (len .Bins) 1 }}
<div class="panel" role="complementary">
<div class="panel-heading" role="heading">
conflicting packages
</div>
<div class="panel-body">
<ul class="list-group list-group-flush">
{{ range $idx, $man := .Bins }}
<li class="list-group-item
{{- if eq $man.Package.Binarypkg $.Meta.Package.Binarypkg }} active{{- end -}}
">
<a href="/{{ $man.ServingPath }}.html">{{ $man.Package.Binarypkg }}</a>
</li>
{{ end }}
</ul>
</div>
</div>
{{ end }}


{{ .Content }}

{{ template "footer" . }}
`
var manpageerrorContent = `{{ template "header" . }}

<div class="panel" role="complementary">
<div class="panel-heading" role="heading">
links
</div>
<div class="panel-body">
<ul class="list-group list-group-flush">
<li class="list-group-item">
<a href="/{{ .Meta.PermaLink }}">permalink</a>
</li>
<li class="list-group-item">
<a href="https://tracker.debian.org/pkg/{{ .Meta.Package.Binarypkg }}">package tracker</a>
</li>
<li class="list-group-item">
<a href="/{{ .Meta.RawPath }}">raw man page</a>
</li>
</ul>
</div>
</div>

<div class="panel" role="complementary">
<div class="panel-heading" role="heading">
other versions
</div>
<div class="panel-body">
<ul class="list-group list-group-flush">
{{ range $idx, $man := .Suites }}
<li class="list-group-item
{{- if eq $man.Package.Suite $.Meta.Package.Suite }} active{{- end -}}
">
<a href="/{{ $man.ServingPath }}.html">{{ $man.Package.Suite }}</a> <span class="pkgversion">{{ $man.Package.Version }}</span>
</li>
{{ end }}
</ul>
</div>
</div>

{{ if gt (len .Langs) 1 }}
<div class="panel" role="complementary">
<div class="panel-heading" role="heading">
other languages
</div>
<div class="panel-body">
<ul class="list-group list-group-flush">
{{ range $idx, $man := .Langs }}
<li class="list-group-item
{{- if eq $man.Language $.Meta.Language }} active{{- end -}}
">
<a href="/{{ $man.ServingPath }}.html">{{ DisplayLang $man.LanguageTag }}</a>
</li>
{{ end }}
</ul>
</div>
</div>
{{ end }}

{{ if gt (len .Sections) 1 }}
<div class="panel" role="complementary">
<div class="panel-heading" role="heading">
other sections
</div>
<div class="panel-body">
<ul class="list-group list-group-flush">
{{ range $idx, $man := .Sections }}
<li class="list-group-item
{{- if eq $man.Section $.Meta.Section }} active{{- end -}}
">
<a href="/{{ $man.ServingPath }}.html">{{ $man.Section }} (<span title="{{ LongSection $man.Section }}">{{ ShortSection $man.Section }}</span>)</a>
</li>
{{ end }}
</ul>
</div>
</div>
{{ end }}

{{ if gt (len .Bins) 1 }}
<div class="panel" role="complementary">
<div class="panel-heading" role="heading">
conflicting packages
</div>
<div class="panel-body">
<ul class="list-group list-group-flush">
{{ range $idx, $man := .Bins }}
<li class="list-group-item
{{- if eq $man.Package.Binarypkg $.Meta.Package.Binarypkg }} active{{- end -}}
">
<a href="/{{ $man.ServingPath }}.html">{{ $man.Package.Binarypkg }}</a>
</li>
{{ end }}
</ul>
</div>
</div>
{{ end }}

<p>
  Sorry, the manpage could not be rendered!
</p>

<p>
  Error message: {{ .Error }}
</p>

{{ template "footer" . }}
`
var contentsContent = `{{ template "header" . }}

<h1>Binary packages containing manpages in Debian {{ .Suite }}</h1>

<ul>
{{ range $idx, $dir := .Bins }}
  <li><a href="/{{ $.Suite }}/{{ $dir}}/index.html">{{ $dir }}</a></li>
{{ end }}
</ul>

{{ template "footer" . }}
`
var pkgindexContent = `{{ template "header" . }}

<h1>Manpages of {{ .First.Package.Binarypkg }} in Debian {{ .First.Package.Suite }}</h1>
  
<ul>
{{ range $idx, $fn := .Mans }}
  {{ with $m := index $.ManpageByName $fn }}
    <li><a href="/{{ $m.ServingPath }}.html">{{ $m.Name }}({{ $m.Section }})</a></li>
  {{ end }}
{{ end }}
</ul>

{{ template "footer" . }}
`
var indexContent = `{{ template "header" . }}

<h1>Debian Manpages</h1>

<p>
  You’re looking at a complete repository of all manpages contained in
  Debian.<br>There are a couple of different ways to use this
  repository:
</p>

<ol>
  <li>
    <form method="GET" action="http://man.localhost/jump">
      Directly jump to manpage:
      <input type="text" name="q" autofocus="autofocus" placeholder="manpage name">
      <input type="submit" value="Jump to manpage">
    </form>
  </li>

  <li>
    In your browser address bar, type enough characters of manpages.debian.org,<br>
    press TAB, enter the manpage name, hit ENTER.
  </li>

  <li>
    Navigate to the manpage’s address, using this URL schema:<br>
    <code>/&lt;suite&gt;/&lt;binarypackage&gt;/&lt;manpage&gt;.&lt;section&gt;.&lt;language&gt;.html</code><br>
    Any part (except <code>&lt;manpage&gt;</code>) can be omitted, and you will be redirected according to our best guess.
  </li>

  <li>
    Browse the repository index:
    <ul>
      {{ range $idx, $suite := .Suites }}
      <li>
	<a href="/contents-{{ $suite }}.html">Debian {{ $suite }}</a>
      </li>
      {{ end }}
    </ul>
  </li>

</ol>

<p>
  If you have more questions, check out the <a href="/about.html">About</a> page or the <a href="/faq.html">FAQ</a>.
</p>

{{ template "footer" . }}
`
var faqContent = `{{ template "header" . }}

<h1>FAQ</h1>

{{ template "footer" . }}
`
