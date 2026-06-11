export type StackFrame = {
	functionName: string | null;
	location: string;
	isLibrary: boolean;
};

export type FrameGroup = {
	type: 'app';
	frame: StackFrame;
} | {
	type: 'library';
	frames: StackFrame[];
	packageName: string;
};

export type ParsedStackTrace = {
	errorMessage: string;
	groups: FrameGroup[];
};

function extractPackageName(location: string): string {
	const nodeModulesMatch = location.match(/node_modules\/([^/]+)/);
	if (nodeModulesMatch) return nodeModulesMatch[1];

	const nodeInternalMatch = location.match(/^node:[a-z_]+/);
	if (nodeInternalMatch) return nodeInternalMatch[0];

	const dartMatch = location.match(/^(package:[^/]+|dart:[^/]+)/);
	if (dartMatch) return dartMatch[1];

	return 'library';
}

export function parseStackTrace(raw: string): ParsedStackTrace {
	const lines = raw.split('\n');
	const frames: StackFrame[] = [];
	let firstFrameIndex = -1;
	let firstFuncNameIndex = -1;

	const locationPattern = /^\s*.+:\d+:\d+$/;

	for (let i = 0; i < lines.length; i++) {
		if (locationPattern.test(lines[i])) {
			const location = lines[i].trim();
			let functionName: string | null = null;

			for (let j = i - 1; j >= 0; j--) {
				const prevLine = lines[j].trim();
				if (prevLine === '') continue;
				if (!locationPattern.test(lines[j])) {
					const messageLike =
						firstFrameIndex === -1 && !prevLine.endsWith('()') && prevLine.includes(': ');
					if (!messageLike) {
						functionName = prevLine;
						if (firstFrameIndex === -1) firstFuncNameIndex = j;
					}
				}
				break;
			}

			if (firstFrameIndex === -1) firstFrameIndex = i;

			frames.push({
				functionName,
				location,
				isLibrary: location.includes('node_modules') ||
					/^(package:flutter\/|dart:|package:collection\/|node:)/.test(location)
			});
		}
	}

	const errorEndIndex = firstFrameIndex === -1
		? lines.length
		: firstFuncNameIndex !== -1 ? firstFuncNameIndex : firstFrameIndex;
	const errorMessage = firstFrameIndex === -1
		? raw.trim()
		: lines.slice(0, errorEndIndex).join('\n').trim();

	const groups: FrameGroup[] = [];

	for (let i = 0; i < frames.length; i++) {
		const frame = frames[i];

		if (!frame.isLibrary) {
			groups.push({ type: 'app', frame });
		} else {
			const libraryFrames: StackFrame[] = [frame];
			while (i + 1 < frames.length && frames[i + 1].isLibrary) {
				i++;
				libraryFrames.push(frames[i]);
			}
			groups.push({
				type: 'library',
				frames: libraryFrames,
				packageName: extractPackageName(libraryFrames[0].location)
			});
		}
	}

	return { errorMessage, groups };
}
