// plainText drops the markdown markers catalog notes carry, keeping the words
// and the line breaks, since the app shows notes as plain text.
export function plainText(md: string): string {
	return md
		.replace(/^ {0,3}#{1,6}\s+/gm, '')
		.replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
		.replace(/(\*\*|__)(?=\S)(.+?)\1/g, '$2')
		.replace(/(^|[^\w*])\*(?=\S)([^*\n]+?)\*(?!\w)/g, '$1$2')
		.replace(/(^|\W)_(?=\S)([^_\n]+?)_(?!\w)/g, '$1$2')
		.replace(/`([^`\n]+)`/g, '$1');
}
