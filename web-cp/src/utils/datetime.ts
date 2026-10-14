export function formatSeconds(totalSeconds : number) {
    console.log(totalSeconds)
    const days = Math.floor(totalSeconds / 86400);
    const hours = Math.floor((totalSeconds % 86400) / 3600);
    const minutes = Math.floor((totalSeconds % 3600) / 60);
    const seconds = Math.floor(totalSeconds %  60);

    const parts = [];
    if (days>0) parts.push(`${days}j`);
    if (hours>0 || hours > 0) parts.push(`${hours}h`);
    if (minutes>0 ||  hours > 0 || days > 0) parts.push(`${minutes}m`);
    parts.push(`${seconds}s`);

    return parts.join(' ');
}