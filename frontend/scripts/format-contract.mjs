import { readFile, writeFile } from 'node:fs/promises';
import ts from 'typescript';

// Preserve repository declaration spacing in reproducible OpenAPI output.
const path = process.argv[2];
const source = await readFile(path, 'utf8');
const file = ts.createSourceFile(path, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
const insertions = new Set();

for (const declaration of file.statements) {
	if (
		!ts.isInterfaceDeclaration(declaration) &&
		!ts.isTypeAliasDeclaration(declaration) &&
		!ts.isClassDeclaration(declaration) &&
		!ts.isEnumDeclaration(declaration)
	) {
		continue;
	}

	const comments = ts.getLeadingCommentRanges(source, declaration.getFullStart()) ?? [];
	const start = comments[0]?.pos ?? declaration.getStart(file);
	const lineStart = source.lastIndexOf('\n', start - 1) + 1;
	if (lineStart === 0) {
		continue;
	}

	const previousStart = source.lastIndexOf('\n', lineStart - 2) + 1;
	if (source.slice(previousStart, lineStart - 1).trim()) {
		insertions.add(lineStart);
	}
}

let formatted = source;
for (const position of [...insertions].sort((left, right) => right - left)) {
	formatted = formatted.slice(0, position) + '\n' + formatted.slice(position);
}

await writeFile(path, formatted);
