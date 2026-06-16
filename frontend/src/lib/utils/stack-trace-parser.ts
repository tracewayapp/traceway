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

function isLibraryLocation(location: string): boolean {
	return (
		location.includes('node_modules') ||
		/^node:/.test(location) ||
		/^dart:/.test(location) ||
		/^third_party\//.test(location) ||
		/^package:(flutter(\/|_)|collection\/)/.test(location) ||
		/SnapshotInstructions\+0x/.test(location)
	);
}

function extractPackageName(location: string): string {
	const nodeModulesMatch = location.match(/node_modules\/([^/]+)/);
	if (nodeModulesMatch) return nodeModulesMatch[1];

	const nodeInternalMatch = location.match(/^node:[a-z_]+/);
	if (nodeInternalMatch) return nodeInternalMatch[0];

	const dartMatch = location.match(/^(package:[^/]+|dart:[^/]+)/);
	if (dartMatch) return dartMatch[1];

	if (/^third_party\//.test(location)) return 'dart sdk';
	if (/SnapshotInstructions\+0x/.test(location)) return 'unresolved';

	return 'library';
}

export function parseStackTrace(raw: string): ParsedStackTrace {
	const lines = raw.split('\n');
	const frames: StackFrame[] = [];
	let firstFrameIndex = -1;
	let messageEndIndex = -1;

	const locationPattern = /^\s*.+:\d+:\d+$/;
	const dartFramePattern = /^\s*#\d+\s+(.+?)\s+\((.+)\)\s*$/;
	const dartUnresolvedPattern = /^\s*#\d+\s+(\S+SnapshotInstructions\+0x[0-9a-fA-F]+)\s*$/;

	for (let i = 0; i < lines.length; i++) {
		const dartMatch = lines[i].match(dartFramePattern);
		if (dartMatch) {
			const location = dartMatch[2].trim();
			if (firstFrameIndex === -1) {
				firstFrameIndex = i;
				messageEndIndex = i;
			}
			frames.push({
				functionName: dartMatch[1].trim(),
				location,
				isLibrary: isLibraryLocation(location)
			});
			continue;
		}

		const unresolvedMatch = lines[i].match(dartUnresolvedPattern);
		if (unresolvedMatch) {
			if (firstFrameIndex === -1) {
				firstFrameIndex = i;
				messageEndIndex = i;
			}
			frames.push({ functionName: null, location: unresolvedMatch[1].trim(), isLibrary: true });
			continue;
		}

		if (locationPattern.test(lines[i])) {
			const location = lines[i].trim();
			let functionName: string | null = null;
			let funcNameIndex = -1;

			for (let j = i - 1; j >= 0; j--) {
				const prevLine = lines[j].trim();
				if (prevLine === '') continue;
				if (!locationPattern.test(lines[j])) {
					const messageLike =
						firstFrameIndex === -1 && !prevLine.endsWith('()') && prevLine.includes(': ');
					if (!messageLike) {
						functionName = prevLine;
						funcNameIndex = j;
					}
				}
				break;
			}

			if (firstFrameIndex === -1) {
				firstFrameIndex = i;
				messageEndIndex = funcNameIndex !== -1 ? funcNameIndex : i;
			}

			frames.push({ functionName, location, isLibrary: isLibraryLocation(location) });
		}
	}

	const errorMessage = firstFrameIndex === -1
		? raw.trim()
		: lines.slice(0, messageEndIndex).join('\n').trim();

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
