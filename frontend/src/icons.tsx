// Interface icons from the HackerNoon Pixel Icon Library (MIT,
// https://pixeliconlibrary.com). Every icon goes through this file, so the set
// can be swapped in one place. The app logo stays Lucide's library-big (see
// Logo in App.tsx): it's a brand mark, and this set has no bookshelf.
//
// The icons are pixel art on a 24px grid with 1px lines, so they are sharpest
// at 24px (or 48px). Use those sizes where there is room: the header and the
// main buttons. Smaller icons are drawn smoothed, which keeps thin lines from
// breaking up when they fall between screen pixels.
import book from "@hackernoon/pixel-icon-library/icons/SVG/regular/book.svg?raw";
import bulletList from "@hackernoon/pixel-icon-library/icons/SVG/regular/bullet-list.svg?raw";
import check from "@hackernoon/pixel-icon-library/icons/SVG/regular/check.svg?raw";
import cog from "@hackernoon/pixel-icon-library/icons/SVG/regular/cog.svg?raw";
import copy from "@hackernoon/pixel-icon-library/icons/SVG/regular/copy.svg?raw";
import externalLink from "@hackernoon/pixel-icon-library/icons/SVG/regular/external-link.svg?raw";
import grid from "@hackernoon/pixel-icon-library/icons/SVG/regular/grid.svg?raw";
import lock from "@hackernoon/pixel-icon-library/icons/SVG/regular/lock.svg?raw";
import logout from "@hackernoon/pixel-icon-library/icons/SVG/regular/logout.svg?raw";
import minus from "@hackernoon/pixel-icon-library/icons/SVG/regular/minus.svg?raw";
import questionCircle from "@hackernoon/pixel-icon-library/icons/SVG/regular/question-circle.svg?raw";
import refresh from "@hackernoon/pixel-icon-library/icons/SVG/regular/refresh.svg?raw";
import thumbsDown from "@hackernoon/pixel-icon-library/icons/SVG/regular/thumbsdown.svg?raw";
import thumbsUp from "@hackernoon/pixel-icon-library/icons/SVG/regular/thumbsup.svg?raw";
import times from "@hackernoon/pixel-icon-library/icons/SVG/regular/times.svg?raw";

export type IconProps = { className?: string };
export type Icon = (props: IconProps) => React.ReactElement;

/** Keeps only the shapes inside the file's <svg> element. */
function shapes(svg: string): string {
  return svg
    .replace(/<\?xml[^>]*\?>/, "")
    .replace(/<svg[^>]*>/, "")
    .replace(/<\/svg>\s*$/, "");
}

function pixelIcon(svg: string): Icon {
  // The markup comes from the bundled icon files, never from user data.
  const html = { __html: shapes(svg) };
  return function PixelIcon({ className = "h-4 w-4" }: IconProps) {
    return (
      <svg
        viewBox="0 0 24 24"
        fill="currentColor"
        aria-hidden
        focusable="false"
        className={`pixel-icon shrink-0 ${className}`}
        dangerouslySetInnerHTML={html}
      />
    );
  };
}

export const BookIcon = pixelIcon(book);
export const CardsIcon = pixelIcon(grid);
export const CheckIcon = pixelIcon(check);
export const CloseIcon = pixelIcon(times);
export const CopyIcon = pixelIcon(copy);
export const ExternalLinkIcon = pixelIcon(externalLink);
export const HelpIcon = pixelIcon(questionCircle);
export const KeyIcon = pixelIcon(lock); // the set has no key; a lock says "secret" too
export const MixedIcon = pixelIcon(minus);
export const SettingsIcon = pixelIcon(cog);
export const SignOutIcon = pixelIcon(logout);
export const SyncIcon = pixelIcon(refresh);
export const TableIcon = pixelIcon(bulletList);
export const ThumbsDownIcon = pixelIcon(thumbsDown);
export const ThumbsUpIcon = pixelIcon(thumbsUp);
